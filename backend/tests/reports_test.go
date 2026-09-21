package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// Причина жалобы из контракта: ею жалуются там, где сам текст значения
// не имеет (specs/008-reports.md, «API / контракт данных»).
const reportReason = "Это не про дачу"

// Причина жалобы на комментарий — оттуда же.
const commentReportReason = "Грубость"

// Предел длины причины — тот же, что у комментария и подписи, и в тех же
// единицах: символы, а не байты (ФТ-2).
const reportReasonLimit = 1000

// --- Представления из базы ------------------------------------------------

// reportRow — жалоба, как она лежит в таблице `reports`. Наружу сервис
// жалоб не отдаёт вовсе — ни ручки, ни числа у поста, — поэтому
// проверить записанное можно только в базе (ФТ-10, ADR-0017,
// «Модель данных»).
type reportRow struct {
	ReporterID string
	Reason     *string
}

// --- Хелперы --------------------------------------------------------------

// reportPost жалуется на пост. body передаётся как есть: тестам про
// непригодное тело нужен не объект, а что угодно, а nil означает
// «тела нет вовсе».
func reportPost(t *testing.T, baseURL, token, postID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, baseURL+"/posts/"+postID+"/report", token, body)
}

// reportPostReason жалуется на пост с этой причиной — так это делает
// приложение.
func reportPostReason(t *testing.T, baseURL, token, postID, reason string) *http.Response {
	t.Helper()
	return reportPost(t, baseURL, token, postID, map[string]any{"reason": reason})
}

// reportComment жалуется на комментарий по адресу его поста: пара «пост
// и комментарий» должна сойтись (ФТ-8).
func reportComment(t *testing.T, baseURL, token, postID, commentID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, baseURL+"/posts/"+postID+"/comments/"+commentID+"/report", token, body)
}

// reportCommentReason жалуется на комментарий с этой причиной.
func reportCommentReason(t *testing.T, baseURL, token, postID, commentID, reason string) *http.Response {
	t.Helper()
	return reportComment(t, baseURL, token, postID, commentID, map[string]any{"reason": reason})
}

// reportRawBody отправляет тело жалобы как есть, не собирая его из
// объекта: телу, которое вовсе не JSON, объекта в Go не соответствует.
func reportRawBody(t *testing.T, address, token, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, address, strings.NewReader(body))
	if err != nil {
		t.Fatalf("не удалось собрать запрос на жалобу: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос на жалобу не прошёл: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// requireAccepted требует, чтобы жалоба была принята и ответ был пустым:
// 202 и ничего больше — сделает по жалобе что-то человек и позже (ФТ-9,
// ADR-0017).
func requireAccepted(t *testing.T, resp *http.Response) {
	t.Helper()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("на жалобу ожидался статус 202, получен %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("не удалось прочитать тело ответа на жалобу: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("ответ на жалобу обязан быть пустым, пришло %q", body)
	}
}

// reasonPtr — причина, которая в жалобе есть: в базе она может быть null,
// поэтому ожидание тоже указатель.
func reasonPtr(reason string) *string {
	return &reason
}

// reportsOn читает жалобы на пост или на комментарий прямо из базы
// и заодно требует, чтобы в каждой было заполнено ровно одно из post_id
// и comment_id («Модель данных», check).
func reportsOn(t *testing.T, column, id, where string) []reportRow {
	t.Helper()

	pool := connect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx,
		`SELECT reporter_id::text, reason, post_id IS NULL, comment_id IS NULL
		 FROM reports WHERE `+column+` = $1 ORDER BY created_at, id`, id)
	if err != nil {
		t.Fatalf("%s: не удалось прочитать жалобы из базы: %v", where, err)
	}
	defer rows.Close()

	found := make([]reportRow, 0, 2)
	for rows.Next() {
		var (
			row         reportRow
			postNull    bool
			commentNull bool
		)
		if err := rows.Scan(&row.ReporterID, &row.Reason, &postNull, &commentNull); err != nil {
			t.Fatalf("%s: жалоба из базы не разобралась: %v", where, err)
		}
		if postNull == commentNull {
			t.Errorf("%s: в жалобе заполнено не ровно одно из post_id и comment_id", where)
		}
		found = append(found, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: чтение жалоб из базы оборвалось: %v", where, err)
	}

	return found
}

// postReports — жалобы на этот пост.
func postReports(t *testing.T, postID, where string) []reportRow {
	t.Helper()
	return reportsOn(t, "post_id", postID, where)
}

// commentReports — жалобы на этот комментарий.
func commentReports(t *testing.T, commentID, where string) []reportRow {
	t.Helper()
	return reportsOn(t, "comment_id", commentID, where)
}

// formatReports печатает жалобы так, чтобы в сообщении об ошибке было
// видно и кто пожаловался, и что написал.
func formatReports(reports []reportRow) string {
	parts := make([]string, 0, len(reports))
	for _, report := range reports {
		reason := "<без причины>"
		if report.Reason != nil {
			reason = "«" + *report.Reason + "»"
		}
		parts = append(parts, report.ReporterID+": "+reason)
	}

	return "[" + strings.Join(parts, "; ") + "]"
}

// requireReports требует, чтобы в базе лежали ровно эти жалобы.
func requireReports(t *testing.T, got, want []reportRow, where string) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: ожидались жалобы %s, в базе %s", where, formatReports(want), formatReports(got))
	}
}

// sortReports упорядочивает жалобы по тому, кто пожаловался: когда
// жалуются разные люди, важно, что записаны обе, а не в каком порядке.
func sortReports(reports []reportRow) []reportRow {
	sorted := append([]reportRow(nil), reports...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ReporterID < sorted[j].ReporterID })

	return sorted
}

// totalReports — сколько жалоб в таблице вообще: отвергнутая жалоба
// не записывается никуда.
func totalReports(t *testing.T) int {
	t.Helper()
	return countSQL(t, "SELECT count(*) FROM reports")
}

// requireNoReports требует, чтобы отвергнутая жалоба не оставила следа.
func requireNoReports(t *testing.T, where string) {
	t.Helper()

	if count := totalReports(t); count != 0 {
		t.Errorf("%s: жалоба не принята, а в таблице reports лежит строк: %d", where, count)
	}
}

// snapshotJSON читает тело ответа целиком и разбирает его, не зная полей:
// тесту важно не то, какие поля он знает, а то, что ответ не изменился
// ни в одном из них.
func snapshotJSON(t *testing.T, resp *http.Response, where string) any {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: ожидался статус 200, получен %d", where, resp.StatusCode)
	}

	var value any
	if err := json.Unmarshal(rawJSON(t, resp), &value); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON: %v", where, err)
	}

	return value
}

// requireUnchanged требует, чтобы ответ остался тем же самым: жалоба
// ничего не прячет и ничего не меняет (ФТ-1, «Лента, лайки
// и комментарии после жалобы»).
func requireUnchanged(t *testing.T, before, after any, what string) {
	t.Helper()

	if !reflect.DeepEqual(before, after) {
		t.Errorf("%s изменился после жалобы:\nбыло:  %v\nстало: %v", what, before, after)
	}
}

// requireNoReportFields требует, чтобы в объекте не появилось ни одного
// поля про жалобы: ни числа жалоб, ни признака «я жаловался» — жалоба
// сигнал, а не публичная метка (ФТ-10, ADR-0017).
func requireNoReportFields(t *testing.T, raw []byte, where string) {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON: %v", where, err)
	}

	for field := range fields {
		if strings.Contains(strings.ToLower(field), "report") {
			t.Errorf("%s: в ответе есть поле %q — жалобы не видны никому, кроме владельца сервиса", where, field)
		}
	}
}

// postToReport публикует пост с одной фотографией: жалобе всё равно, что
// на ней, важен только сам пост.
func postToReport(t *testing.T, baseURL, token string) postPayload {
	t.Helper()
	return postWithPhotos(t, baseURL, token, 1)
}

// --- POST /api/posts/{postId}/report: пожаловаться на пост ----------------

// Жалоба на чужой пост: 202 с пустым телом, жалоба записана, а пост
// остался на месте и виден всем — и тому, кто пожаловался, и автору
// («Жалоба на чужой пост», «Жалоба с причиной», ФТ-1, ФТ-9,
// пользовательский сценарий, шаги 3-4).
func TestReportOnAnotherUsersPostIsAcceptedAndLeavesItInPlace(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)

	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, reportReason))

	requireReports(t, postReports(t, post.ID, "после жалобы"),
		[]reportRow{{reporterID, reasonPtr(reportReason)}}, "после жалобы на чужой пост")

	requirePostAlive(t, baseURL, reporter, post.ID, "пожаловавшийся после жалобы")
	requirePostAlive(t, baseURL, author, post.ID, "автор после жалобы")
}

// Жалоба без тела запроса — это жалоба: 202 и причины нет («Жалоба без
// тела запроса», ФТ-2, «API»).
func TestReportOnPostWithoutBodyHasNoReason(t *testing.T) {
	bodies := map[string]any{
		"тела нет вовсе": nil,
		"пустой объект":  map[string]any{},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			author, _ := signIn(t, baseURL, phonePretty)
			reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

			post := postToReport(t, baseURL, author)

			requireAccepted(t, reportPost(t, baseURL, reporter, post.ID, body))

			requireReports(t, postReports(t, post.ID, "после жалобы без причины"),
				[]reportRow{{reporterID, nil}}, "жалоба без причины")
		})
	}
}

// Причина из одних пробелов — это отсутствие причины, а не ошибка: 202
// и причины нет («Причина из одних пробелов», «API»).
func TestReportReasonOfOnlySpacesIsNoReason(t *testing.T) {
	reasons := map[string]string{
		"пустая строка":      "",
		"один пробел":        " ",
		"несколько пробелов": "     ",
		"переводы строки":    "\n\n\n",
		"пробелы и переводы": "  \n \t \r\n  ",
		"табуляции":          "\t\t",
	}

	for caseName, reason := range reasons {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			author, _ := signIn(t, baseURL, phonePretty)
			reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

			post := postToReport(t, baseURL, author)

			requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, reason))

			requireReports(t, postReports(t, post.ID, "после жалобы с пустой причиной"),
				[]reportRow{{reporterID, nil}}, "причина из одних пробелов")
		})
	}
}

// Пробелы по краям причины обрезаются — как у подписи и комментария
// («API»).
func TestReportReasonIsTrimmed(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)

	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, "  \n\t "+reportReason+" \n  "))

	requireReports(t, postReports(t, post.ID, "после жалобы с пробелами по краям"),
		[]reportRow{{reporterID, reasonPtr(reportReason)}}, "причина с пробелами по краям")
}

// Ровно 1000 символов после обрезки — 202: предел включительный, как
// у комментария, и считается он в символах, а не в байтах (ФТ-2).
func TestReportReasonOfExactlyOneThousandCharactersIsAccepted(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)

	// В ягоде четыре байта: если бы длину считали в байтах, такая
	// причина не прошла бы.
	reason := repeatRunes("🍓", reportReasonLimit)
	if len(reason) <= reportReasonLimit {
		t.Fatalf("тест собран неправильно: в причине %d байт", len(reason))
	}

	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, " "+reason+" "))

	requireReports(t, postReports(t, post.ID, "после жалобы с предельной причиной"),
		[]reportRow{{reporterID, reasonPtr(reason)}}, "причина ровно в 1000 символов")
}

// Причина длиннее 1000 символов — 400 invalid_reason, и жалоба
// не записана («Причина длиннее 1000 символов»).
func TestReportReasonLongerThanOneThousandCharactersIsRejected(t *testing.T) {
	reasons := map[string]string{
		"на один символ длиннее":                      repeatRunes("а", reportReasonLimit+1),
		"на один символ длиннее, символ многобайтный": repeatRunes("🍓", reportReasonLimit+1),
		"вдвое длиннее предела":                       repeatRunes("грубость ", reportReasonLimit*2),
	}

	for caseName, reason := range reasons {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			author, _ := signIn(t, baseURL, phonePretty)
			reporter, _ := signIn(t, baseURL, otherPhonePretty)

			post := postToReport(t, baseURL, author)

			requireError(t, reportPostReason(t, baseURL, reporter, post.ID, reason),
				http.StatusBadRequest, "invalid_reason")

			requireNoReports(t, "после слишком длинной причины")
			requirePostAlive(t, baseURL, reporter, post.ID, "после слишком длинной причины")
		})
	}
}

// Тело прислано, но не разбирается — 400 invalid_request, и жалоба
// не записана («Тело прислано, но не разбирается»).
func TestReportOnPostRejectsMalformedBody(t *testing.T) {
	bodies := map[string]any{
		"не объект запроса":        "это не объект запроса",
		"причина не строка":        map[string]any{"reason": 42},
		"причина — массив":         map[string]any{"reason": []string{"а", "б"}},
		"причина вложена в объект": map[string]any{"reason": map[string]any{"reason": reportReason}},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			author, _ := signIn(t, baseURL, phonePretty)
			reporter, _ := signIn(t, baseURL, otherPhonePretty)

			post := postToReport(t, baseURL, author)

			requireError(t, reportPost(t, baseURL, reporter, post.ID, body),
				http.StatusBadRequest, "invalid_request")

			requireNoReports(t, "после непригодного тела")
		})
	}

	// И тело, которое вовсе не JSON, — то же самое.
	raw := map[string]string{
		"не JSON вовсе":   "это не про дачу",
		"оборванный JSON": `{"reason": "это не про дачу"`,
	}

	for caseName, body := range raw {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			author, _ := signIn(t, baseURL, phonePretty)
			reporter, _ := signIn(t, baseURL, otherPhonePretty)

			post := postToReport(t, baseURL, author)

			requireError(t, reportRawBody(t, baseURL+"/posts/"+post.ID+"/report", reporter, body),
				http.StatusBadRequest, "invalid_request")

			requireNoReports(t, "после непригодного тела")
		})
	}
}

// На своё не жалуются: своё удаляют. Жалоба на свой пост — 403 own_post,
// и жалоба не записана («Свой пост», ФТ-3).
func TestReportOnOwnPostIsForbidden(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToReport(t, baseURL, token)

	requireError(t, reportPostReason(t, baseURL, token, post.ID, reportReason),
		http.StatusForbidden, "own_post")

	requireNoReports(t, "после жалобы на свой пост")
	requirePostAlive(t, baseURL, token, post.ID, "после жалобы на свой пост")

	// И жалоба без тела на свой пост — тоже 403.
	requireError(t, reportPost(t, baseURL, token, post.ID, nil), http.StatusForbidden, "own_post")
	requireNoReports(t, "после жалобы без причины на свой пост")
}

// Тело смотрят последним: у своего поста причина длиннее предела даёт
// тот же 403 own_post, а не 400 invalid_reason («Свой пост и причина
// длиннее 1000 символов», ФТ-7).
func TestReportOnOwnPostIsForbiddenBeforeTheBodyIsLookedAt(t *testing.T) {
	bodies := map[string]any{
		"причина длиннее 1000 символов":              map[string]any{"reason": repeatRunes("а", reportReasonLimit+1)},
		"причина длиннее 1000 символов, с пробелами": map[string]any{"reason": "  " + repeatRunes("🍓", reportReasonLimit+1) + "  "},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			post := postToReport(t, baseURL, token)

			requireError(t, reportPost(t, baseURL, token, post.ID, body), http.StatusForbidden, "own_post")

			requireNoReports(t, "после жалобы на свой пост со слишком длинной причиной")
		})
	}
}

// Тело, которое вообще не разбирается, отвергает сам контракт — раньше
// всех проверок: и у своего поста, и у несуществующего, и без токена
// ответ один, 400 invalid_request («Свой пост и тело, которое
// не разбирается», ФТ-7, ADR-0001).
func TestReportWithUnparseableBodyIsRejectedBeforeAnyCheck(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	other, _ := signIn(t, baseURL, otherPhonePretty)

	own := postToReport(t, baseURL, author)
	foreign := postToReport(t, baseURL, other)
	ownComment := commentOf(t, baseURL, author, foreign.ID, commentText)
	foreignComment := commentOf(t, baseURL, other, own.ID, commentText)

	const unparseable = "это не объект запроса"

	cases := map[string]func() *http.Response{
		"свой пост": func() *http.Response {
			return reportPost(t, baseURL, author, own.ID, unparseable)
		},
		"чужой пост": func() *http.Response {
			return reportPost(t, baseURL, author, foreign.ID, unparseable)
		},
		"несуществующий пост": func() *http.Response {
			return reportPost(t, baseURL, author, unknownID, unparseable)
		},
		"пост, чей идентификатор не UUID": func() *http.Response {
			return reportPost(t, baseURL, author, notAnID, unparseable)
		},
		"свой комментарий": func() *http.Response {
			return reportComment(t, baseURL, author, foreign.ID, ownComment.ID, unparseable)
		},
		"чужой комментарий": func() *http.Response {
			return reportComment(t, baseURL, author, own.ID, foreignComment.ID, unparseable)
		},
		"несуществующий комментарий": func() *http.Response {
			return reportComment(t, baseURL, author, own.ID, unknownCommentID, unparseable)
		},
		// Даже токен здесь не первый: разбирать нечего ещё до того, как
		// сервис посмотрит, кто пришёл.
		"без токена": func() *http.Response {
			return reportPost(t, baseURL, "", own.ID, unparseable)
		},
	}

	for caseName, request := range cases {
		t.Run(caseName, func(t *testing.T) {
			requireError(t, request(), http.StatusBadRequest, "invalid_request")
		})
	}

	requireNoReports(t, "после непригодных тел")
}

// Повторная жалоба того же человека на тот же пост принимается так же,
// как первая, но второй записи не появляется, и первая причина
// не переписывается: владелец сервиса мог её уже прочитать («Повторная
// жалоба на тот же пост», ФТ-4).
func TestReportOnTheSamePostTwiceKeepsOneReportAndTheFirstReason(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)

	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, reportReason))

	first := []reportRow{{reporterID, reasonPtr(reportReason)}}
	requireReports(t, postReports(t, post.ID, "после первой жалобы"), first, "после первой жалобы")

	// Вторая жалоба с другой причиной: ответ тот же.
	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, "и вообще тут реклама"))
	requireReports(t, postReports(t, post.ID, "после второй жалобы"), first, "после второй жалобы с другой причиной")

	// И третья, уже без причины: первая причина всё та же.
	requireAccepted(t, reportPost(t, baseURL, reporter, post.ID, nil))
	requireReports(t, postReports(t, post.ID, "после третьей жалобы"), first, "после третьей жалобы без причины")

	if count := totalReports(t); count != 1 {
		t.Errorf("после трёх жалоб одного человека на один пост в таблице строк: %d, а должна быть одна", count)
	}
}

// Жалобы разных людей на один пост записываются каждая: сколько людей
// пожаловалось — это первое, что увидит владелец сервиса («Жалобы двух
// разных людей на один пост», ФТ-5).
func TestReportsOfDifferentPeopleOnOnePostAreBothRecorded(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	first, firstID := signIn(t, baseURL, otherPhonePretty)
	second, secondID := signIn(t, baseURL, thirdPhonePretty)

	post := postToReport(t, baseURL, author)

	requireAccepted(t, reportPostReason(t, baseURL, first, post.ID, reportReason))
	requireAccepted(t, reportPostReason(t, baseURL, second, post.ID, commentReportReason))

	want := sortReports([]reportRow{
		{firstID, reasonPtr(reportReason)},
		{secondID, reasonPtr(commentReportReason)},
	})

	requireReports(t, sortReports(postReports(t, post.ID, "после двух жалоб")), want, "жалобы двух разных людей")
}

// Уникальность — на пару «пост и пожаловавшийся»: тот же человек
// жалуется на другой пост, и это отдельная жалоба (ФТ-4, ФТ-5,
// «Модель данных»).
func TestReportsOfOnePersonOnDifferentPostsAreBothRecorded(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	first := postToReport(t, baseURL, author)
	second := postToReport(t, baseURL, author)

	requireAccepted(t, reportPostReason(t, baseURL, reporter, first.ID, reportReason))
	requireAccepted(t, reportPost(t, baseURL, reporter, second.ID, nil))

	requireReports(t, postReports(t, first.ID, "жалоба на первый пост"),
		[]reportRow{{reporterID, reasonPtr(reportReason)}}, "жалоба на первый пост")
	requireReports(t, postReports(t, second.ID, "жалоба на второй пост"),
		[]reportRow{{reporterID, nil}}, "жалоба на второй пост")
}

// Поста нет — 404 post_not_found. Идентификатор, который вовсе не похож
// на UUID, отвечает так же: сервис таких не выдавал («Несуществующий
// пост», «Идентификатор поста не похож на UUID»).
func TestReportOnUnknownPostIsNotFound(t *testing.T) {
	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 notAnID,
	}

	for caseName, id := range ids {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			reporter, _ := signIn(t, baseURL, phonePretty)

			requireError(t, reportPostReason(t, baseURL, reporter, id, reportReason),
				http.StatusNotFound, "post_not_found")
			// И без тела, и со слишком длинной причиной — то же самое:
			// сначала сервис отвечает, есть ли на что жаловаться (ФТ-7).
			requireError(t, reportPost(t, baseURL, reporter, id, nil), http.StatusNotFound, "post_not_found")
			requireError(t, reportPostReason(t, baseURL, reporter, id, repeatRunes("а", reportReasonLimit+1)),
				http.StatusNotFound, "post_not_found")

			requireNoReports(t, "после жалобы на несуществующий пост")
		})
	}
}

// Существование проверяется раньше, чем «своё ли»: жалоба на свой пост,
// которого уже нет, — 404 post_not_found, а не 403 («Жалоба
// на несуществующий свой пост», ФТ-7).
func TestReportOnOwnDeletedPostIsNotFoundNotForbidden(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToReport(t, baseURL, token)

	requireDeleted(t, deletePost(t, baseURL, token, post.ID))

	requireError(t, reportPostReason(t, baseURL, token, post.ID, reportReason),
		http.StatusNotFound, "post_not_found")

	requireNoReports(t, "после жалобы на свой удалённый пост")
}

// --- POST /api/posts/{postId}/comments/{commentId}/report -----------------

// Жалоба на чужой комментарий: 202, комментарий на месте, число
// комментариев у поста не изменилось («Жалоба на чужой комментарий»,
// ФТ-1, пользовательский сценарий, шаг 5).
func TestReportOnAnotherUsersCommentIsAcceptedAndLeavesItInPlace(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	commenter, _ := signIn(t, baseURL, otherPhonePretty)
	reporter, reporterID := signIn(t, baseURL, thirdPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, commenter, post.ID, commentText)

	requireAccepted(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, commentReportReason))

	requireReports(t, commentReports(t, comment.ID, "после жалобы на комментарий"),
		[]reportRow{{reporterID, reasonPtr(commentReportReason)}}, "жалоба на чужой комментарий")

	// Комментарий никуда не делся, и его видят все.
	for who, token := range map[string]string{"автор поста": author, "автор комментария": commenter, "пожаловавшийся": reporter} {
		requireCommentTexts(t, baseURL, token, post.ID, []string{commentText}, who+" после жалобы")
		requireCommentCountEverywhere(t, baseURL, token, post.ID, 1, who+" после жалобы")
	}
}

// Чужой комментарий под своим постом убирают жалобой, а не удалением:
// 202 («Чужой комментарий под своим постом», ФТ-1).
func TestReportOnAnotherUsersCommentUnderOwnPostIsAccepted(t *testing.T) {
	baseURL := startAPI(t)

	author, authorID := signIn(t, baseURL, phonePretty)
	commenter, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, commenter, post.ID, commentText)

	requireAccepted(t, reportCommentReason(t, baseURL, author, post.ID, comment.ID, commentReportReason))

	requireReports(t, commentReports(t, comment.ID, "после жалобы владельца поста"),
		[]reportRow{{authorID, reasonPtr(commentReportReason)}}, "жалоба на чужой комментарий под своим постом")

	requireCommentCountEverywhere(t, baseURL, author, post.ID, 1, "после жалобы на чужой комментарий")
}

// На свой комментарий не жалуются — его удаляют: 403 own_comment.
// Под своим постом и под чужим одинаково («Свой комментарий», «Свой
// комментарий под чужим постом», ФТ-3).
func TestReportOnOwnCommentIsForbidden(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	commenter, _ := signIn(t, baseURL, otherPhonePretty)

	ownPost := postToReport(t, baseURL, commenter)
	foreignPost := postToReport(t, baseURL, author)

	underOwn := commentOf(t, baseURL, commenter, ownPost.ID, commentText)
	underForeign := commentOf(t, baseURL, commenter, foreignPost.ID, commentText)

	cases := map[string][2]string{
		"свой комментарий под своим постом": {ownPost.ID, underOwn.ID},
		"свой комментарий под чужим постом": {foreignPost.ID, underForeign.ID},
	}

	for caseName, pair := range cases {
		t.Run(caseName, func(t *testing.T) {
			requireError(t, reportCommentReason(t, baseURL, commenter, pair[0], pair[1], commentReportReason),
				http.StatusForbidden, "own_comment")
			// Тело смотрят последним: слишком длинная причина ответа
			// не меняет (ФТ-7).
			requireError(t, reportCommentReason(t, baseURL, commenter, pair[0], pair[1],
				repeatRunes("а", reportReasonLimit+1)), http.StatusForbidden, "own_comment")
			// И жалоба без причины на своё — тот же отказ.
			requireError(t, reportComment(t, baseURL, commenter, pair[0], pair[1], nil),
				http.StatusForbidden, "own_comment")
		})
	}

	requireNoReports(t, "после жалоб на свои комментарии")
	requireCommentCountEverywhere(t, baseURL, commenter, ownPost.ID, 1, "после жалобы на свой комментарий")
	requireCommentCountEverywhere(t, baseURL, commenter, foreignPost.ID, 1, "после жалобы на свой комментарий")
}

// Повторная жалоба на тот же комментарий: 202, второй записи нет,
// первая причина не переписана («Повторная жалоба на тот же
// комментарий», ФТ-4).
func TestReportOnTheSameCommentTwiceKeepsOneReport(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, author, post.ID, commentText)

	requireAccepted(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, commentReportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, "а ещё и грубость"))
	requireAccepted(t, reportComment(t, baseURL, reporter, post.ID, comment.ID, nil))

	requireReports(t, commentReports(t, comment.ID, "после трёх жалоб"),
		[]reportRow{{reporterID, reasonPtr(commentReportReason)}}, "повторная жалоба на комментарий")

	if count := totalReports(t); count != 1 {
		t.Errorf("после трёх жалоб на один комментарий в таблице строк: %d, а должна быть одна", count)
	}
}

// Жалоба на комментарий подаётся по адресу его поста: комментарий,
// существующий, но лежащий под другим постом, — 404 comment_not_found
// («Комментарий, лежащий под другим постом», ФТ-8).
func TestReportOnCommentByTheAddressOfAnotherPostIsNotFound(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, _ := signIn(t, baseURL, otherPhonePretty)

	first := postToReport(t, baseURL, author)
	second := postToReport(t, baseURL, author)

	comment := commentOf(t, baseURL, author, first.ID, commentText)

	requireError(t, reportCommentReason(t, baseURL, reporter, second.ID, comment.ID, commentReportReason),
		http.StatusNotFound, "comment_not_found")

	requireNoReports(t, "после жалобы по адресу чужого поста")
	requireCommentTexts(t, baseURL, reporter, first.ID, []string{commentText}, "комментарий после чужого адреса")
}

// Пост проверяется первым: жалоба на комментарий несуществующего поста —
// 404 post_not_found, даже если сам комментарий существует («Комментарий
// у несуществующего поста», ФТ-7).
func TestReportOnCommentOfUnknownPostIsPostNotFound(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, author, post.ID, commentText)

	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 notAnID,
	}

	for caseName, postID := range ids {
		t.Run(caseName, func(t *testing.T) {
			requireError(t, reportCommentReason(t, baseURL, reporter, postID, comment.ID, commentReportReason),
				http.StatusNotFound, "post_not_found")
			requireError(t, reportComment(t, baseURL, reporter, postID, unknownCommentID, nil),
				http.StatusNotFound, "post_not_found")
		})
	}

	// И на свой комментарий у несуществующего поста — тоже про пост:
	// существование проверяется раньше, чем «своё ли».
	requireError(t, reportCommentReason(t, baseURL, author, unknownID, comment.ID, commentReportReason),
		http.StatusNotFound, "post_not_found")

	requireNoReports(t, "после жалобы у несуществующего поста")
}

// Комментария нет — 404 comment_not_found. Идентификатор, не похожий
// на UUID, отвечает так же («Идентификатор комментария не похож
// на UUID»).
func TestReportOnUnknownCommentIsNotFound(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)

	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownCommentID,
		"вовсе не UUID":                 notAnID,
	}

	for caseName, commentID := range ids {
		t.Run(caseName, func(t *testing.T) {
			requireError(t, reportCommentReason(t, baseURL, reporter, post.ID, commentID, commentReportReason),
				http.StatusNotFound, "comment_not_found")
			// Тело смотрят последним и здесь (ФТ-7).
			requireError(t, reportCommentReason(t, baseURL, reporter, post.ID, commentID,
				repeatRunes("а", reportReasonLimit+1)), http.StatusNotFound, "comment_not_found")
			requireError(t, reportComment(t, baseURL, reporter, post.ID, commentID, nil),
				http.StatusNotFound, "comment_not_found")
		})
	}

	// Удалённый комментарий — тот же несуществующий.
	comment := commentOf(t, baseURL, author, post.ID, commentText)
	requireDeleted(t, deleteComment(t, baseURL, author, post.ID, comment.ID))
	requireError(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, commentReportReason),
		http.StatusNotFound, "comment_not_found")

	requireNoReports(t, "после жалобы на несуществующий комментарий")
}

// Причина у жалобы на комментарий живёт по тем же правилам, что
// и у жалобы на пост: длиннее 1000 символов — 400 invalid_reason, одни
// пробелы — 202 без причины, непригодное тело — 400 invalid_request
// (ФТ-2, «Ошибки»).
func TestReportOnCommentChecksItsReasonTheSameWay(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, author, post.ID, commentText)

	requireError(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID,
		repeatRunes("🍓", reportReasonLimit+1)), http.StatusBadRequest, "invalid_reason")
	requireNoReports(t, "после слишком длинной причины у комментария")

	requireError(t, reportComment(t, baseURL, reporter, post.ID, comment.ID, map[string]any{"reason": 42}),
		http.StatusBadRequest, "invalid_request")
	requireError(t, reportRawBody(t, baseURL+"/posts/"+post.ID+"/comments/"+comment.ID+"/report",
		reporter, "грубость"), http.StatusBadRequest, "invalid_request")
	requireNoReports(t, "после непригодного тела у комментария")

	// Ровно 1000 символов принимаются, а одни пробелы — это отсутствие
	// причины.
	requireAccepted(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, "   \n\t  "))
	requireReports(t, commentReports(t, comment.ID, "после жалобы с пустой причиной"),
		[]reportRow{{reporterID, nil}}, "причина из одних пробелов у комментария")
}

// --- Доступ ---------------------------------------------------------------

// Жаловаться может только вошедший: без токена и с недействительным
// токеном — 401 unauthorized, и жалоба не записана («Жалоба без токена»,
// ФТ-13).
func TestReportRequiresValidToken(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	other, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, other, post.ID, commentText)

	tokens := map[string]string{
		"без токена":        "",
		"неизвестный токен": "этого-токена-сервис-не-выдавал",
	}

	for caseName, badToken := range tokens {
		t.Run("жалоба на пост, "+caseName, func(t *testing.T) {
			requireError(t, reportPostReason(t, baseURL, badToken, post.ID, reportReason),
				http.StatusUnauthorized, "unauthorized")
		})
		t.Run("жалоба на комментарий, "+caseName, func(t *testing.T) {
			requireError(t, reportCommentReason(t, baseURL, badToken, post.ID, comment.ID, commentReportReason),
				http.StatusUnauthorized, "unauthorized")
		})
	}

	requireNoReports(t, "после жалоб без токена")

	// И после выхода из сессии токен уже не годится (specs/001-auth.md).
	if resp := signOut(t, baseURL, other); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на выход ожидался статус 204, получен %d", resp.StatusCode)
	}
	requireError(t, reportPostReason(t, baseURL, other, post.ID, reportReason),
		http.StatusUnauthorized, "unauthorized")
	requireNoReports(t, "после жалобы вышедшего")
}

// Токен проверяется первым: жалоба без токена на свой пост — 401,
// а не 403, и на несуществующий пост — тоже 401, а не 404 («Жалоба
// на свой пост без токена», ФТ-7).
func TestReportWithoutTokenIsUnauthorizedBeforeAnyOtherCheck(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToReport(t, baseURL, token)
	comment := commentOf(t, baseURL, token, post.ID, commentText)

	tokens := map[string]string{
		"без токена":        "",
		"неизвестный токен": "этого-токена-сервис-не-выдавал",
	}

	for caseName, badToken := range tokens {
		// Свой пост: владелец токена его автор, но токена нет.
		t.Run("свой пост, "+caseName, func(t *testing.T) {
			requireError(t, reportPostReason(t, baseURL, badToken, post.ID, reportReason),
				http.StatusUnauthorized, "unauthorized")
		})
		t.Run("свой комментарий, "+caseName, func(t *testing.T) {
			requireError(t, reportCommentReason(t, baseURL, badToken, post.ID, comment.ID, commentReportReason),
				http.StatusUnauthorized, "unauthorized")
		})
		t.Run("несуществующий пост, "+caseName, func(t *testing.T) {
			requireError(t, reportPostReason(t, baseURL, badToken, unknownID, reportReason),
				http.StatusUnauthorized, "unauthorized")
			requireError(t, reportPostReason(t, baseURL, badToken, notAnID, reportReason),
				http.StatusUnauthorized, "unauthorized")
		})
		t.Run("несуществующий комментарий, "+caseName, func(t *testing.T) {
			requireError(t, reportCommentReason(t, baseURL, badToken, post.ID, unknownCommentID, reportReason),
				http.StatusUnauthorized, "unauthorized")
		})
		// И слишком длинная причина без токена — тоже 401: тело смотрят
		// последним.
		t.Run("слишком длинная причина, "+caseName, func(t *testing.T) {
			requireError(t, reportPostReason(t, baseURL, badToken, post.ID,
				repeatRunes("а", reportReasonLimit+1)), http.StatusUnauthorized, "unauthorized")
		})
	}

	requireNoReports(t, "после запросов без токена")
}

// --- Жалоба ничего не меняет и никому не видна ---------------------------

// После жалобы не меняется ничего: ни пост, ни лента, ни лайки,
// ни комментарии. Жалоба — сигнал владельцу сервиса, и больше ничего
// («Лента, лайки и комментарии после жалобы», ФТ-1).
func TestReportChangesNeitherThePostNorTheFeedNorLikesNorComments(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, _ := signIn(t, baseURL, otherPhonePretty)
	viewer, _ := signIn(t, baseURL, thirdPhonePretty)

	post := postToReport(t, baseURL, author)
	other := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, viewer, post.ID, commentText)

	// Лайк от того, кто потом пожалуется, и от постороннего.
	if resp := likePost(t, baseURL, reporter, post.ID); resp.StatusCode != http.StatusOK {
		t.Fatalf("на лайк ожидался статус 200, получен %d", resp.StatusCode)
	}
	if resp := likePost(t, baseURL, viewer, post.ID); resp.StatusCode != http.StatusOK {
		t.Fatalf("на лайк ожидался статус 200, получен %d", resp.StatusCode)
	}

	// Снимок всего, что видно снаружи, — глазами каждого из троих.
	type view struct {
		post     any
		feed     any
		comments any
	}

	tokens := map[string]string{"автор": author, "пожаловавшийся": reporter, "посторонний": viewer}

	before := map[string]view{}
	for who, token := range tokens {
		before[who] = view{
			post:     snapshotJSON(t, fetchPost(t, baseURL, token, post.ID), who+": пост до жалобы"),
			feed:     snapshotJSON(t, fetchFeed(t, baseURL, token, nil), who+": лента до жалобы"),
			comments: snapshotJSON(t, fetchComments(t, baseURL, token, post.ID), who+": комментарии до жалобы"),
		}
	}

	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, reportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, commentReportReason))

	for who, token := range tokens {
		after := view{
			post:     snapshotJSON(t, fetchPost(t, baseURL, token, post.ID), who+": пост после жалобы"),
			feed:     snapshotJSON(t, fetchFeed(t, baseURL, token, nil), who+": лента после жалобы"),
			comments: snapshotJSON(t, fetchComments(t, baseURL, token, post.ID), who+": комментарии после жалобы"),
		}

		requireUnchanged(t, before[who].post, after.post, who+": пост")
		requireUnchanged(t, before[who].feed, after.feed, who+": лента")
		requireUnchanged(t, before[who].comments, after.comments, who+": комментарии")
	}

	// И соседний пост на месте: жалоба не трогает даже того, на что
	// не жаловались.
	requirePostAlive(t, baseURL, reporter, other.ID, "соседний пост после жалобы")
	requireLikesEverywhere(t, baseURL, reporter, post.ID, 2, true, "пожаловавшийся после жалобы")
	requireCommentCountEverywhere(t, baseURL, author, post.ID, 1, "автор после жалобы")
}

// Жалоба не показывается нигде: ни числа жалоб у поста, ни признака
// «я жаловался», ни списка жалоб. Ручек «мои жалобы» и «жалобы на пост»
// нет (ФТ-10, ADR-0017).
func TestReportIsInvisibleInEveryPublicAnswer(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, author, post.ID, commentText)

	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, reportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, commentReportReason))

	// В посте по своему адресу, в посте из ленты и в комментарии полей
	// про жалобы нет.
	requireNoReportFields(t, rawJSON(t, fetchPost(t, baseURL, reporter, post.ID)), "пост после жалобы")

	var feed feedRawPayload
	decode(t, fetchFeed(t, baseURL, reporter, nil), &feed)
	if len(feed.Items) == 0 {
		t.Fatal("в ленте нет постов, а пост опубликован")
	}
	for _, item := range feed.Items {
		requireNoReportFields(t, item, "пост ленты после жалобы")
	}

	var comments struct {
		Items []json.RawMessage `json:"items"`
	}
	decode(t, fetchComments(t, baseURL, reporter, post.ID), &comments)
	if len(comments.Items) != 1 {
		t.Fatalf("под постом ожидался один комментарий, получено %d", len(comments.Items))
	}
	requireNoReportFields(t, comments.Items[0], "комментарий после жалобы")

	// Ручки, которая отдала бы жалобы, нет: ни своих, ни чужих.
	addresses := map[string]string{
		"жалобы на пост":        baseURL + "/posts/" + post.ID + "/report",
		"жалобы на комментарий": baseURL + "/posts/" + post.ID + "/comments/" + comment.ID + "/report",
		"мои жалобы":            baseURL + "/reports",
	}

	for name, address := range addresses {
		t.Run(name, func(t *testing.T) {
			resp := do(t, http.MethodGet, address, reporter, nil)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				t.Errorf("GET %s ответил %d: жалобы никому не отдаются", address, resp.StatusCode)
			}
		})
	}
}

// --- Жизнь жалобы ---------------------------------------------------------

// Жалоба живёт только вместе с тем, на что пожаловались: автор удалил
// свой пост — ушли и жалобы на него, и жалобы на его комментарии («Пост
// удалён после жалобы на него», «Пост удалён после жалобы на его
// комментарий», ФТ-6, ADR-0007).
func TestReportsDisappearWithTheirPost(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, author, post.ID, commentText)

	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, reportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, commentReportReason))

	requireReports(t, postReports(t, post.ID, "перед удалением поста"),
		[]reportRow{{reporterID, reasonPtr(reportReason)}}, "жалоба на пост перед удалением")
	requireReports(t, commentReports(t, comment.ID, "перед удалением поста"),
		[]reportRow{{reporterID, reasonPtr(commentReportReason)}}, "жалоба на комментарий перед удалением")

	requireDeleted(t, deletePost(t, baseURL, author, post.ID))

	if count := totalReports(t); count != 0 {
		t.Errorf("пост удалён, а жалоб в таблице осталось %d — жалоба живёт только вместе с тем, на что пожаловались", count)
	}
}

// Комментарий удалён после жалобы на него — жалобы нет вместе с ним,
// а жалоба на сам пост остаётся: ушло только то, на что смотреть больше
// не на что («Комментарий удалён после жалобы на него», ФТ-6).
func TestReportOnCommentDisappearsWithTheComment(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	reporter, reporterID := signIn(t, baseURL, otherPhonePretty)

	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, author, post.ID, commentText)

	requireAccepted(t, reportPostReason(t, baseURL, reporter, post.ID, reportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, reporter, post.ID, comment.ID, commentReportReason))

	requireDeleted(t, deleteComment(t, baseURL, author, post.ID, comment.ID))

	requireReports(t, commentReports(t, comment.ID, "после удаления комментария"),
		[]reportRow{}, "жалоба на удалённый комментарий")
	requireReports(t, postReports(t, post.ID, "после удаления комментария"),
		[]reportRow{{reporterID, reasonPtr(reportReason)}}, "жалоба на пост после удаления комментария")

	if count := totalReports(t); count != 1 {
		t.Errorf("после удаления комментария в таблице строк: %d, а должна остаться одна — жалоба на пост", count)
	}
}
