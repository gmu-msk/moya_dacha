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

// normalizeTag приводит тэг к виду, в котором он хранится (требование 2):
// без пробелов по краям и `#` в начале, в нижнем регистре. Пустой —
// пустая строка без ошибки (требование 3).
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

// errTooManyTagsOnPost — после схлопывания повторов тэгов больше десяти.
var errTooManyTagsOnPost = errors.New("тэгов больше десяти")

// normalizeTags нормализует тэги поста: пустые пропускает, повторы
// схлопывает по первому появлению (требования 2–5).
func normalizeTags(raw []string) ([]string, error) {
	tags := make([]string, 0, len(raw))
	for _, item := range raw {
		tag, err := normalizeTag(item)
		if err != nil {
			return nil, err
		}
		if tag != "" && !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	if len(tags) > MaxPostTags {
		return nil, errTooManyTagsOnPost
	}
	return tags, nil
}

// tagsError — ответ на ошибку normalizeTags.
func tagsError(err error) gen.Error {
	if errors.Is(err, errTooManyTagsOnPost) {
		return errTooManyTags
	}
	return errInvalidTag
}

// execer — пул или транзакция: тэги пишутся и при создании поста, внутри
// его транзакции, и правкой.
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

// SetPostTags заменяет тэги своего поста (требования 9–10). Порядок
// проверок как у правки подписи: пост, «своё ли», потом тэги.
func (s *Server) SetPostTags(ctx context.Context, request gen.SetPostTagsRequestObject) (gen.SetPostTagsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SetPostTags401JSONResponse(errUnauthorized), nil
	}

	author, err := s.postAuthor(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.SetPostTags404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if author != current.user.Id {
		return gen.SetPostTags403JSONResponse(errNotYourPostEdit), nil
	}
	// Пустой список снимает тэги, а тело без списка — ошибка: иначе
	// промах клиента молча стёр бы их.
	if request.Body == nil || request.Body.Tags == nil {
		return gen.SetPostTags400JSONResponse(errInvalidTagsEdit), nil
	}

	tags, err := normalizeTags(*request.Body.Tags)
	if err != nil {
		return gen.SetPostTags400JSONResponse(tagsError(err)), nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := writeTags(ctx, tx, request.PostId, tags); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	return gen.SetPostTags200JSONResponse(post), nil
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

	excluded := map[string]bool{}
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

	var text string
	if request.Params.Text != nil {
		text = *request.Params.Text
	}

	items := suggestTags(popular, firstRunes(text, MaxCaptionLength), excluded)
	return gen.GetTagSuggestions200JSONResponse(gen.TagSuggestions{Items: items}), nil
}

// suggestTags выбирает подсказки из тэгов сообщества и словаря
// (требования 16–18).
func suggestTags(popular []tagCandidate, text string, excluded map[string]bool) []string {
	byTag := make(map[string]*tagCandidate, len(popular)+len(starterTags))
	candidates := make([]*tagCandidate, 0, len(popular)+len(starterTags))
	add := func(c tagCandidate) {
		if excluded[c.tag] {
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
		return !unicode.IsLetter(symbol) && !unicode.IsDigit(symbol) && symbol != '-' && symbol != '_'
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

// Ошибки тэгов (specs/028-post-tags.md, «API»).
var (
	errInvalidTag = gen.Error{
		Code:    "invalid_tag",
		Message: "Тэг — одно слово из букв и цифр, до 30 знаков",
	}
	errTooManyTags = gen.Error{
		Code:    "too_many_tags",
		Message: "Не больше 10 тэгов у поста",
	}
	errInvalidTagsEdit = gen.Error{
		Code:    "invalid_request",
		Message: "Пришёл пустой запрос. Выберите тэги и сохраните ещё раз",
	}
)
