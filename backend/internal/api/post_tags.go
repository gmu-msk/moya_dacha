// Тэги у поста и подсказки тэгов: specs/028-post-tags.md.
package api

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// MaxTagLength — сколько знаков в тэге (требование 1).
const MaxTagLength = 30

// MaxPostTags — сколько тэгов у поста (требование 5).
const MaxPostTags = 10

// MaxTagSuggestions — сколько тэгов подсказывается за раз (требование 15).
const MaxTagSuggestions = 5

// starterTags — стартовый словарь подсказок (требование 16): с ним
// подсказки есть и у сообщества, где тэгов ещё никто не ставил.
var starterTags = []string{
	"поделюсь", "советы", "вопрос", "дневник", "урожай", "рассада",
	"теплица", "полив", "удобрения", "вредители", "болезни", "заготовки",
	"цветы", "томаты", "огурцы", "перцы", "картофель", "капуста", "кабачки",
	"тыква", "морковь", "свёкла", "лук", "чеснок", "зелень", "клубника",
	"малина", "смородина", "яблоня", "груша", "вишня", "слива", "виноград",
}

// errTagInvalid — тэг не одно слово до 30 знаков.
var errTagInvalid = errors.New("тэг не подходит")

// normalizeTag приводит тэг к виду, в котором он хранится (требования 2, 13):
// без пробелов по краям и `#` в начале, в нижнем регистре. Пустой —
// пустая строка без ошибки.
func normalizeTag(raw string) (string, error) {
	tag := strings.ToLower(strings.TrimLeft(strings.TrimSpace(raw), "#"))
	if tag == "" {
		return "", nil
	}
	if utf8.RuneCountInString(tag) > MaxTagLength {
		return "", errTagInvalid
	}
	meaningful := false
	for _, symbol := range tag {
		switch {
		case unicode.IsLetter(symbol), unicode.IsDigit(symbol):
			meaningful = true
		case symbol == '-', symbol == '_':
		default:
			return "", errTagInvalid
		}
	}
	if !meaningful {
		return "", errTagInvalid
	}
	return tag, nil
}

// isTagSymbol — знак слова тэга: буква, цифра, `-` или `_` (требование 1).
func isTagSymbol(symbol rune) bool {
	return unicode.IsLetter(symbol) || unicode.IsDigit(symbol) || symbol == '-' || symbol == '_'
}

// captionTags — тэги поста из хэштегов подписи (требования 3–5): `#`
// в начале или после не-знака слова, дальше знаки слова подряд. Слово,
// которое не годится в тэг, — просто текст; повторы схлопываются, берутся
// первые десять.
func captionTags(caption string) []string {
	tags := []string{}
	symbols := []rune(caption)
	for i := 0; i < len(symbols) && len(tags) < MaxPostTags; i++ {
		if symbols[i] != '#' || (i > 0 && isTagSymbol(symbols[i-1])) {
			continue
		}
		end := i + 1
		for end < len(symbols) && isTagSymbol(symbols[end]) {
			end++
		}
		if end == i+1 {
			continue
		}
		tag, err := normalizeTag(string(symbols[i+1 : end]))
		if err == nil && !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
		i = end - 1
	}
	return tags
}

// execer — пул или транзакция: тэги пишутся при создании поста и при
// правке подписи, внутри их транзакций.
type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// writeTags заменяет тэги поста целиком.
func writeTags(ctx context.Context, db execer, postID string, tags []string) error {
	if _, err := db.Exec(ctx, `DELETE FROM post_tags WHERE post_id = $1`, postID); err != nil {
		return err
	}
	if len(tags) == 0 {
		return nil
	}
	_, err := db.Exec(ctx, `
		INSERT INTO post_tags (post_id, tag, position)
		SELECT $1, tag, position FROM unnest($2::text[]) WITH ORDINALITY AS t(tag, position)`,
		postID, tags)
	return err
}

// attachTags раскладывает тэги по постам одним запросом на страницу,
// как attachMedia. У поста без тэгов — пустой список (требование 7).
func (s *Server) attachTags(ctx context.Context, posts []gen.Post) error {
	if len(posts) == 0 {
		return nil
	}

	ids := make([]string, len(posts))
	for i, post := range posts {
		ids[i] = post.Id
	}

	rows, err := s.db.Query(ctx, `
		SELECT post_id, tag FROM post_tags
		WHERE post_id = ANY($1::uuid[]) ORDER BY post_id, position`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	byPost := make(map[string][]string, len(posts))
	for rows.Next() {
		var postID, tag string
		if err := rows.Scan(&postID, &tag); err != nil {
			return err
		}
		byPost[postID] = append(byPost[postID], tag)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range posts {
		tags := byPost[posts[i].Id]
		if tags == nil {
			tags = []string{}
		}
		posts[i].Tags = &tags
	}
	return nil
}

// tagCandidate — тэг, который можно подсказать.
type tagCandidate struct {
	tag string
	// posts — на скольких видимых смотрящему постах он стоит.
	posts int
	// starter — место в стартовом словаре; -1 — тэга там нет.
	starter int
	// word — номер первого слова текста, совпавшего с тэгом; -1 — нет.
	word int
}

// GetTagSuggestions подсказывает до пяти тэгов черновику поста
// (требования 15–19): найденные в подписи, популярные, словарь.
func (s *Server) GetTagSuggestions(ctx context.Context, request gen.GetTagSuggestionsRequestObject) (gen.GetTagSuggestionsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetTagSuggestions401JSONResponse(errUnauthorized), nil
	}

	var text string
	if request.Params.Text != nil {
		text = firstRunes(*request.Params.Text, MaxCaptionLength)
	}

	// Набираемое начало тэга: не годится — как без него.
	var prefix string
	if request.Params.Prefix != nil {
		if normalized, err := normalizeTag(*request.Params.Prefix); err == nil {
			prefix = normalized
		}
	}

	// Хэштеги черновика уже стали бы тэгами — их не подсказываем, кроме
	// набираемого прямо сейчас.
	excluded := map[string]bool{}
	for _, tag := range captionTags(text) {
		if tag != prefix {
			excluded[tag] = true
		}
	}
	if request.Params.Exclude != nil {
		for _, raw := range *request.Params.Exclude {
			if tag, err := normalizeTag(raw); err == nil && tag != "" {
				excluded[tag] = true
			}
		}
	}

	// Популярность — только по постам, видимым смотрящему: тэг чужого
	// поста «только мне» не подсказывается.
	rows, err := s.db.Query(ctx, `
		SELECT t.tag, count(*) FROM post_tags t JOIN posts p ON p.id = t.post_id
		WHERE `+postVisibleTo("$1")+`
		GROUP BY t.tag`, current.user.Id)
	if err != nil {
		return nil, err
	}
	popular, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (tagCandidate, error) {
		c := tagCandidate{starter: -1, word: -1}
		err := row.Scan(&c.tag, &c.posts)
		return c, err
	})
	if err != nil {
		return nil, err
	}

	items := suggestTags(popular, text, prefix, excluded)
	return gen.GetTagSuggestions200JSONResponse(gen.TagSuggestions{Items: items}), nil
}

// suggestTags выбирает подсказки из тэгов сообщества и словаря
// (требования 15–18); непустой prefix оставляет тэги на это начало.
func suggestTags(popular []tagCandidate, text, prefix string, excluded map[string]bool) []string {
	byTag := make(map[string]*tagCandidate, len(popular)+len(starterTags))
	candidates := make([]*tagCandidate, 0, len(popular)+len(starterTags))
	add := func(c tagCandidate) {
		if excluded[c.tag] || !strings.HasPrefix(c.tag, prefix) {
			return
		}
		if known, ok := byTag[c.tag]; ok {
			known.posts = max(known.posts, c.posts)
			known.starter = max(known.starter, c.starter)
			return
		}
		item := c
		byTag[c.tag] = &item
		candidates = append(candidates, &item)
	}
	for _, c := range popular {
		add(c)
	}
	for i, tag := range starterTags {
		add(tagCandidate{tag: tag, starter: i, word: -1})
	}

	for i, word := range textWords(text) {
		for _, c := range candidates {
			if c.word < 0 && wordMatchesTag(word, c.tag) {
				c.word = i
			}
		}
	}

	slices.SortStableFunc(candidates, func(a, b *tagCandidate) int {
		// Найденные в тексте — первыми, по порядку слов.
		if (a.word >= 0) != (b.word >= 0) {
			if a.word >= 0 {
				return -1
			}
			return 1
		}
		if a.word != b.word {
			return cmp.Compare(a.word, b.word)
		}
		// Дальше тэги сообщества по популярности и алфавиту, потом
		// словарь по своему порядку.
		if (a.posts > 0) != (b.posts > 0) {
			if a.posts > 0 {
				return -1
			}
			return 1
		}
		if a.posts > 0 {
			return cmp.Or(cmp.Compare(b.posts, a.posts), strings.Compare(a.tag, b.tag))
		}
		return cmp.Compare(a.starter, b.starter)
	})

	items := make([]string, 0, MaxTagSuggestions)
	for _, c := range candidates[:min(len(candidates), MaxTagSuggestions)] {
		items = append(items, c.tag)
	}
	return items
}

// textWords — слова текста в нижнем регистре: подряд идущие буквы,
// цифры, `-` и `_` (требование 18).
func textWords(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(symbol rune) bool {
		return !isTagSymbol(symbol)
	})
}

// tagEndings — буквы, которые отбрасываются от тэга, чтобы получить
// основу: «груша» → «груш», «томаты» → «томат».
const tagEndings = "аеёиоуыэюяйь"

// wordMatchesTag — совпадает ли слово текста с тэгом (требование 18):
// слово равно тэгу или начинается с его основы (от трёх знаков) и
// длиннее неё не больше чем на два знака.
func wordMatchesTag(word, tag string) bool {
	if word == tag {
		return true
	}
	stem := []rune(tag)
	if last := stem[len(stem)-1]; strings.ContainsRune(tagEndings, last) {
		stem = stem[:len(stem)-1]
	}
	if len(stem) < 3 {
		return false
	}
	letters := []rune(word)
	return len(letters) >= len(stem) &&
		len(letters)-len(stem) <= 2 &&
		string(letters[:len(stem)]) == string(stem)
}

// firstRunes — первые limit знаков строки.
func firstRunes(text string, limit int) string {
	for i := range text {
		if limit == 0 {
			return text[:i]
		}
		limit--
	}
	return text
}

// errInvalidTag — `tag` ленты не годится (specs/028-post-tags.md, «API»).
var errInvalidTag = gen.Error{
	Code:    "invalid_tag",
	Message: "Тэг — одно слово из букв и цифр, до 30 знаков",
}
