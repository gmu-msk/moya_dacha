package tests

// Меню команд и приглашения из лички бота (specs/018-telegram-bot.md,
// требования 23–32).
//
// Telegram — тот же фейк, что в telegram_test.go: он записывает вызовы
// setMyCommands и умеет присылать контакт из телефонной книги. Что код из
// ответа бота правда впускает, тест проверяет так же, как ходит
// приложение: сервисом в режиме приглашений (specs/015-invites.md),
// POST /api/auth/code и /api/auth/session.

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Номера для тестов приглашений из бота: не пересекаются с номерами
// других тестов пакета. Один номер в разных записях (001-auth, ФТ-2).
const (
	tiPretty = "+7 900 818-51-01"
	tiBraces = "+7 (900) 818-51-01"
	tiSpaced = "8 900 818 51 01"
	tiDigits = "79008185101"
	tiStored = "+79008185101"

	tiOtherPretty = "+7 900 818-51-02"
	tiOtherStored = "+79008185102"

	tiThirdStored = "+79008185103"
)

// Тексты из спецификации.
const (
	tiAskPhone    = "Пришлите номер телефона или контакт из телефонной книги."
	tiWrongPhone  = "Это не российский мобильный номер. Пришлите вида +7 900 123-45-67."
	tiUninviteAsk = "Напишите номер: /uninvite +7 900 123-45-67"
	tiListHeader  = "Действующие приглашения:"
	tiListEmpty   = "Действующих приглашений нет."
	tiOnlyPrivate = "Приглашения — только в личке бота."
	tiReplyHeader = "Приглашение в МоюДачу"
)

// tiMenu — меню владельца ровно из требования 24, по порядку.
var tiMenu = []tgCommand{
	{Command: "invite", Description: "Код входа по номеру телефона"},
	{Command: "invites", Description: "Действующие приглашения"},
	{Command: "uninvite", Description: "Отозвать приглашение"},
	{Command: "status", Description: "Сводка сервера"},
	{Command: "inbox", Description: "Входящие задачи"},
	{Command: "approve", Description: "Одобрить задачу"},
}

var tiCodeLine = regexp.MustCompile(`^Код: ([0-9]{4})$`)

// --- Хелперы --------------------------------------------------------------

// tiSetup поднимает сервис в режиме приглашений, привязывает личку
// владельца и запускает бота на той же базе. Сервис стартует первым:
// старт чистит базу.
func tiSetup(t *testing.T) (string, *pgxpool.Pool, *fakeTelegram) {
	t.Helper()

	baseURL, pool := startInvitesAPI(t)
	bindChat(t, "owner", tgOwnerID)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	return baseURL, pool, fake
}

// ownerSays — владелец пишет в личку, тест ждёт ответ и возвращает его.
func ownerSays(t *testing.T, fake *fakeTelegram, text string) tgSent {
	t.Helper()

	before := len(fake.sentTo(tgOwnerID))
	fake.pushOwnerPrivate(text)
	got := fake.waitSent(tgOwnerID, before+1, "ответ на "+text)
	return got[before]
}

// ownerSendsContact — владелец присылает в личку контакт, тест ждёт ответ.
func ownerSendsContact(t *testing.T, fake *fakeTelegram, phone string) tgSent {
	t.Helper()

	before := len(fake.sentTo(tgOwnerID))
	fake.pushContact(tgOwnerID, tgOwnerNick, tgOwnerID, "private", phone)
	got := fake.waitSent(tgOwnerID, before+1, "ответ на контакт "+phone)
	return got[before]
}

// ownerSaysInGroup — владелец пишет в группу, тест ждёт ответ в группе.
func ownerSaysInGroup(t *testing.T, fake *fakeTelegram, chatType, text string) tgSent {
	t.Helper()

	before := len(fake.sentTo(tgGroupID))
	fake.push(tgOwnerID, tgOwnerNick, tgGroupID, chatType, text)
	got := fake.waitSent(tgGroupID, before+1, "ответ в группе на "+text)
	return got[before]
}

// requireText требует ответ ровно этим текстом.
func requireText(t *testing.T, got tgSent, want, what string) {
	t.Helper()

	if got.method != "sendMessage" || strings.TrimSpace(got.text) != want {
		t.Fatalf("%s: ожидался ответ %q, получено: %s", what, want, describeSent([]tgSent{got}))
	}
}

// requireInviteReply требует ответ из требования 26 на этот номер
// и возвращает код из него.
func requireInviteReply(t *testing.T, got tgSent, stored, what string) string {
	t.Helper()

	ls := lines(got.text)
	if got.method != "sendMessage" || len(ls) != 3 || ls[0] != tiReplyHeader || ls[1] != "Номер: "+stored {
		t.Fatalf("%s: ожидался ответ «%s / Номер: %s / Код: NNNN», получено: %s",
			what, tiReplyHeader, stored, describeSent([]tgSent{got}))
	}
	m := tiCodeLine.FindStringSubmatch(ls[2])
	if m == nil {
		t.Fatalf("%s: третья строка должна быть «Код: » и четыре цифры, получено %q", what, ls[2])
	}
	return m[1]
}

// requireCodeSignsIn — код из ответа бота впускает: запрос кода видит
// приглашение, вход по коду проходит (015, ФТ-7, ФТ-10).
func requireCodeSignsIn(t *testing.T, baseURL, phone, code string) {
	t.Helper()

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phone))
	requireSignedIn(t, createSession(t, baseURL, phone, code), "вход по коду из ответа бота")
}

// inviteAttempts — сколько попыток осталось у приглашения на номер;
// ok == false, если приглашения нет.
func inviteAttempts(t *testing.T, pool *pgxpool.Pool, stored string) (int, bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	err := pool.QueryRow(ctx, `SELECT attempts_left FROM invites WHERE phone = $1`, stored).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("не удалось прочитать invites: %v", err)
	}
	return n, true
}

// countInvites — сколько строк в invites.
func countInvites(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM invites`).Scan(&n); err != nil {
		t.Fatalf("не удалось прочитать invites: %v", err)
	}
	return n
}

// requireNoInvites — приглашений в базе нет.
func requireNoInvites(t *testing.T, pool *pgxpool.Pool, what string) {
	t.Helper()
	if n := countInvites(t, pool); n != 0 {
		t.Fatalf("%s: приглашение выдаваться не должно, а в invites строк: %d", what, n)
	}
}

// requireNoCodeSent — бот никуда не отправил кода приглашения.
func requireNoCodeSent(t *testing.T, fake *fakeTelegram, what string) {
	t.Helper()
	for _, s := range fake.allSent() {
		if strings.Contains(s.text, "Код:") {
			t.Fatalf("%s: бот отправил код приглашения: %s", what, describeSent([]tgSent{s}))
		}
	}
}

// requireOwnerMenu — вызов setMyCommands со scope личка владельца
// и списком ровно из требования 24.
func requireOwnerMenu(t *testing.T, c tgCommands, what string) {
	t.Helper()

	if c.scopeType != "chat" || c.scopeChatID != tgOwnerID {
		t.Fatalf("%s: scope setMyCommands должен быть {type: chat, chat_id: %d}, получен {type: %q, chat_id: %d}",
			what, tgOwnerID, c.scopeType, c.scopeChatID)
	}
	if len(c.list) != len(tiMenu) {
		t.Fatalf("%s: в меню ожидалось %d команд, получено %d: %v", what, len(tiMenu), len(c.list), c.list)
	}
	for i, want := range tiMenu {
		if c.list[i] != want {
			t.Fatalf("%s: команда меню №%d — ожидалась %v, получена %v", what, i+1, want, c.list[i])
		}
	}
}

// --- Меню команд ------------------------------------------------------------

// Личка уже привязана — меню владельцу уходит при запуске Run (ФТ-23, ФТ-24).
func TestTelegramMenuIsSetOnRunWhenOwnerBound(t *testing.T) {
	pool := tgPool(t)
	bindChat(t, "owner", tgOwnerID)
	bindChat(t, "group", tgGroupID)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	fake.waitCommands(1, "меню при запуске")
	settle(t, fake)

	for _, c := range fake.commandCalls() {
		requireOwnerMenu(t, c, "меню при запуске")
	}
}

// Личка не привязана — при запуске меню не отправляется никому; после
// /start владельца уходит в его личку (ФТ-23, ФТ-24).
func TestTelegramMenuIsSetAfterStart(t *testing.T) {
	pool := tgPool(t)
	bindChat(t, "group", tgGroupID) // группе меню не положено
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	settle(t, fake)
	if calls := fake.commandCalls(); len(calls) != 0 {
		t.Fatalf("без привязанной лички setMyCommands вызываться не должен, вызовов: %d (%+v)", len(calls), calls)
	}

	requireText(t, ownerSays(t, fake, "/start"), tgOwnerBound, "/start")
	fake.waitCommands(1, "меню после /start")
	settle(t, fake)

	for _, c := range fake.commandCalls() {
		requireOwnerMenu(t, c, "меню после /start")
	}
}

// /group не отправляет меню в группу: список — только личке владельца (ФТ-24).
func TestTelegramMenuIsNotSetForGroup(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	requireText(t, ownerSaysInGroup(t, fake, "group", "/group"), tgGroupBound, "/group")
	settle(t, fake)

	if calls := fake.commandCalls(); len(calls) != 0 {
		t.Fatalf("после /group setMyCommands вызываться не должен, вызовов: %d (%+v)", len(calls), calls)
	}
}

// Ошибка Telegram на setMyCommands бота не останавливает (ФТ-25).
func TestTelegramMenuErrorDoesNotStopBot(t *testing.T) {
	pool := tgPool(t)
	bindChat(t, "owner", tgOwnerID)
	fake := newFakeTelegram(t)
	fake.setFailCommands(true)
	runBot(t, pool, fake)

	fake.waitCommands(1, "меню при запуске")

	requireText(t, ownerSays(t, fake, "/start"), tgOwnerBound, "/start после ошибки setMyCommands")
	settle(t, fake)
}

// --- /invite с номером --------------------------------------------------------

// /invite и /пригласить с номером в любой записи выдают приглашение,
// ответ — номер и код, код впускает (ФТ-26).
func TestTelegramInviteWithPhone(t *testing.T) {
	cases := []struct{ name, text string }{
		{"плюс семь с пробелами и дефисами", "/invite " + tiPretty},
		{"восьмёрка с пробелами", "/invite " + tiSpaced},
		{"скобки", "/invite " + tiBraces},
		{"по-русски", "/пригласить " + tiPretty},
		{"с ником бота", "/invite@moya_dacha_bot " + tiDigits},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, pool, fake := tiSetup(t)

			code := requireInviteReply(t, ownerSays(t, fake, tc.text), tiStored, tc.text)

			if n, ok := inviteAttempts(t, pool, tiStored); !ok || n != 5 {
				t.Fatalf("после %q ожидалось приглашение на %s с 5 попытками, есть: %v, попыток: %d", tc.text, tiStored, ok, n)
			}
			if n := countInvites(t, pool); n != 1 {
				t.Fatalf("ожидалось одно приглашение, в invites строк: %d", n)
			}
			requireCodeSignsIn(t, baseURL, tiPretty, code)
		})
	}
}

// Повторное /invite на тот же номер заменяет приглашение: старый код
// больше не подходит, новый впускает (ФТ-26, 015 ФТ-4).
func TestTelegramReinviteReplacesCode(t *testing.T) {
	baseURL, pool, fake := tiSetup(t)

	first := requireInviteReply(t, ownerSays(t, fake, "/invite "+tiPretty), tiStored, "первое /invite")

	var second string
	for range 10 {
		second = requireInviteReply(t, ownerSays(t, fake, "/invite "+tiSpaced), tiStored, "повторное /invite")
		if second != first {
			break
		}
	}
	if second == first {
		t.Fatalf("десять повторных /invite подряд выдали тот же код %s: код не случайный", first)
	}
	if n := countInvites(t, pool); n != 1 {
		t.Fatalf("на один номер — одно приглашение, в invites строк: %d", n)
	}

	resp := createSession(t, baseURL, tiPretty, first)
	requireInviteError(t, resp, http.StatusUnauthorized, "invalid_code", "вход старым кодом после повторного /invite")

	requireCodeSignsIn(t, baseURL, tiPretty, second)
}

// --- /invite без номера и контакт ---------------------------------------------

// /invite без номера просит номер; следующий текст владельца — номер
// для приглашения (ФТ-27).
func TestTelegramInviteAsksThenTakesText(t *testing.T) {
	baseURL, _, fake := tiSetup(t)

	requireText(t, ownerSays(t, fake, "/invite"), tiAskPhone, "/invite без номера")

	code := requireInviteReply(t, ownerSays(t, fake, tiSpaced), tiStored, "номер после /invite")
	requireCodeSignsIn(t, baseURL, tiPretty, code)
}

// /пригласить без номера тоже просит номер.
func TestTelegramInviteRussianAsksForPhone(t *testing.T) {
	_, _, fake := tiSetup(t)
	requireText(t, ownerSays(t, fake, "/пригласить"), tiAskPhone, "/пригласить без номера")
}

// /invite без номера, затем контакт из телефонной книги (ФТ-27).
func TestTelegramInviteAsksThenTakesContact(t *testing.T) {
	baseURL, _, fake := tiSetup(t)

	requireText(t, ownerSays(t, fake, "/invite"), tiAskPhone, "/invite без номера")

	code := requireInviteReply(t, ownerSendsContact(t, fake, tiDigits), tiStored, "контакт после /invite")
	requireCodeSignsIn(t, baseURL, tiPretty, code)
}

// Контакт в личку от владельца выдаёт приглашение и без /invite (ФТ-28).
// Telegram присылает номер контакта как есть: и с «+», и без.
func TestTelegramContactWithoutCommandInvites(t *testing.T) {
	for _, phone := range []string{tiDigits, tiStored} {
		t.Run(phone, func(t *testing.T) {
			baseURL, pool, fake := tiSetup(t)

			code := requireInviteReply(t, ownerSendsContact(t, fake, phone), tiStored, "контакт без /invite")
			if n, ok := inviteAttempts(t, pool, tiStored); !ok || n != 5 {
				t.Fatalf("после контакта ожидалось приглашение с 5 попытками, есть: %v, попыток: %d", ok, n)
			}
			requireCodeSignsIn(t, baseURL, tiPretty, code)
		})
	}
}

// Бот ждёт номер одно сообщение: второй номер подряд уже не приглашение (ФТ-27).
func TestTelegramInviteWaitsForOneMessageOnly(t *testing.T) {
	_, pool, fake := tiSetup(t)

	requireText(t, ownerSays(t, fake, "/invite"), tiAskPhone, "/invite без номера")
	requireInviteReply(t, ownerSays(t, fake, tiPretty), tiStored, "номер после /invite")

	fake.pushOwnerPrivate(tiOtherPretty)
	settle(t, fake)

	if _, ok := inviteAttempts(t, pool, tiOtherStored); ok {
		t.Fatal("второй номер после одного /invite не должен становиться приглашением: бот ждёт одно сообщение")
	}
	if n := countInvites(t, pool); n != 1 {
		t.Fatalf("ожидалось одно приглашение, в invites строк: %d", n)
	}
}

// Команда после /invite снимает ожидание и выполняется как обычно;
// следующий текст уже не номер для приглашения (ФТ-27).
func TestTelegramCommandCancelsInviteWait(t *testing.T) {
	_, pool, fake := tiSetup(t)

	requireText(t, ownerSays(t, fake, "/invite"), tiAskPhone, "/invite без номера")

	status := ownerSays(t, fake, "/status")
	ls := lines(status.text)
	if indexOfLine(ls, "Людей: ") < 0 || indexOfLine(ls, "Ошибок за сутки: ") < 0 {
		t.Fatalf("/status после /invite должен ответить сводкой, получено: %s", describeSent([]tgSent{status}))
	}

	fake.pushOwnerPrivate(tiPretty)
	settle(t, fake)

	requireNoInvites(t, pool, "номер после /invite и /status")
	requireNoCodeSent(t, fake, "номер после /invite и /status")
}

// --- Неверный номер -----------------------------------------------------------

// Номер не российский мобильный — ответ из ФТ-29, приглашения нет.
func TestTelegramInviteRejectsWrongPhone(t *testing.T) {
	for _, text := range []string{
		"/invite +1 555 123 4567",
		"/invite +7 495 123-45-67", // городской
		"/invite 12345",
		"/пригласить абв",
	} {
		t.Run(text, func(t *testing.T) {
			_, pool, fake := tiSetup(t)

			requireText(t, ownerSays(t, fake, text), tiWrongPhone, text)
			requireNoInvites(t, pool, text)
		})
	}
}

// Неверный номер после /invite без номера — ответ из ФТ-29, и ожидание
// снимается: следующий верный номер уже не приглашение.
func TestTelegramWrongPhoneAfterInviteEndsWait(t *testing.T) {
	_, pool, fake := tiSetup(t)

	requireText(t, ownerSays(t, fake, "/invite"), tiAskPhone, "/invite без номера")
	requireText(t, ownerSays(t, fake, "+44 20 7946 0958"), tiWrongPhone, "неверный номер после /invite")

	fake.pushOwnerPrivate(tiPretty)
	settle(t, fake)

	requireNoInvites(t, pool, "верный номер после неверного")
}

// Контакт с не российским мобильным — ответ из ФТ-29, приглашения нет.
func TestTelegramContactWithWrongPhone(t *testing.T) {
	_, pool, fake := tiSetup(t)

	requireText(t, ownerSendsContact(t, fake, "+14155550123"), tiWrongPhone, "контакт с американским номером")
	requireNoInvites(t, pool, "контакт с американским номером")
}

// --- /uninvite ------------------------------------------------------------------

// /uninvite и /отозвать с номером отзывают приглашение; ответ тот же,
// и когда приглашения не было (ФТ-30).
func TestTelegramUninvite(t *testing.T) {
	baseURL, pool, fake := tiSetup(t)
	revoked := "Приглашение на " + tiStored + " отозвано."

	requireInviteReply(t, ownerSays(t, fake, "/invite "+tiPretty), tiStored, "/invite")

	requireText(t, ownerSays(t, fake, "/uninvite "+tiSpaced), revoked, "/uninvite")
	if _, ok := inviteAttempts(t, pool, tiStored); ok {
		t.Fatal("после /uninvite приглашение должно быть отозвано")
	}
	resp := requestInviteCode(t, baseURL, tiPretty)
	requireInviteError(t, resp, http.StatusForbidden, "not_invited", "запрос кода после /uninvite")

	requireText(t, ownerSays(t, fake, "/отозвать "+tiPretty), revoked, "/отозвать без приглашения")
}

// /uninvite не трогает приглашения на другие номера.
func TestTelegramUninviteKeepsOtherInvites(t *testing.T) {
	_, pool, fake := tiSetup(t)
	issueInvite(t, pool, tiPretty)
	issueInvite(t, pool, tiOtherPretty)

	requireText(t, ownerSays(t, fake, "/uninvite "+tiPretty), "Приглашение на "+tiStored+" отозвано.", "/uninvite")

	if _, ok := inviteAttempts(t, pool, tiOtherStored); !ok {
		t.Fatal("/uninvite одного номера отозвал приглашение на другой")
	}
}

// /uninvite без номера просит номер, с неверным — ответ из ФТ-29 (ФТ-30).
func TestTelegramUninviteWithoutOrWrongPhone(t *testing.T) {
	_, pool, fake := tiSetup(t)
	issueInvite(t, pool, tiPretty)

	requireText(t, ownerSays(t, fake, "/uninvite"), tiUninviteAsk, "/uninvite без номера")
	requireText(t, ownerSays(t, fake, "/отозвать"), tiUninviteAsk, "/отозвать без номера")
	requireText(t, ownerSays(t, fake, "/uninvite 12345"), tiWrongPhone, "/uninvite с неверным номером")

	if _, ok := inviteAttempts(t, pool, tiStored); !ok {
		t.Fatal("/uninvite без номера или с неверным номером не должен отзывать приглашения")
	}
}

// --- /invites -------------------------------------------------------------------

// Приглашений нет — «Действующих приглашений нет.» (ФТ-31).
func TestTelegramInvitesEmpty(t *testing.T) {
	_, _, fake := tiSetup(t)

	requireText(t, ownerSays(t, fake, "/invites"), tiListEmpty, "/invites без приглашений")
	requireText(t, ownerSays(t, fake, "/приглашения"), tiListEmpty, "/приглашения без приглашений")
}

// Список действующих приглашений: свежие сверху, попытки, время по Москве;
// сгоревшее по попыткам в списке не значится (ФТ-31, 015 ФТ-17).
func TestTelegramInvitesList(t *testing.T) {
	_, pool, fake := tiSetup(t)

	issueInvite(t, pool, tiPretty)
	issueInvite(t, pool, tiOtherPretty)
	issueInvite(t, pool, "+7 900 818-51-03")
	execSQL(t, `UPDATE invites SET created_at = '2026-09-27T18:05:00Z' WHERE phone = $1`, tiStored)
	execSQL(t, `UPDATE invites SET created_at = '2026-09-28T06:30:00Z', attempts_left = 3 WHERE phone = $1`, tiOtherStored)
	execSQL(t, `UPDATE invites SET created_at = '2026-09-28T07:00:00Z', attempts_left = 0 WHERE phone = $1`, tiThirdStored)

	want := []string{
		tiListHeader,
		tiOtherStored + " — попыток 3, выдано 28.09 09:30",
		tiStored + " — попыток 5, выдано 27.09 21:05",
	}
	for _, cmd := range []string{"/invites", "/приглашения"} {
		got := ownerSays(t, fake, cmd)
		ls := lines(got.text)
		if strings.Join(ls, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%s: ожидался список\n%s\nполучено:\n%s", cmd, strings.Join(want, "\n"), got.text)
		}
	}
}

// Приглашение, выданное ботом, видно в /invites с 5 попытками и сегодняшней
// датой по Москве (ФТ-31).
func TestTelegramInvitesListShowsFreshInvite(t *testing.T) {
	_, _, fake := tiSetup(t)

	requireInviteReply(t, ownerSays(t, fake, "/invite "+tiPretty), tiStored, "/invite")
	got := ownerSays(t, fake, "/invites")

	ls := lines(got.text)
	prefix := tiStored + " — попыток 5, выдано "
	if len(ls) != 2 || ls[0] != tiListHeader || !strings.HasPrefix(ls[1], prefix) {
		t.Fatalf("/invites: ожидались «%s» и строка «%s…», получено:\n%s", tiListHeader, prefix, got.text)
	}
	day := time.Now().In(time.FixedZone("MSK", 3*3600)).Format("02.01")
	if !regexp.MustCompile(`^` + regexp.QuoteMeta(prefix+day) + ` [0-2][0-9]:[0-5][0-9]$`).MatchString(ls[1]) {
		t.Fatalf("/invites: время выдачи должно быть «%s ЧЧ:ММ» по Москве, получено %q", day, ls[1])
	}
}

// --- Только в личке владельца ----------------------------------------------------

// В группе на команды приглашений владелец получает «Приглашения — только
// в личке бота.», и ничего не выдаётся и не отзывается (ФТ-32).
func TestTelegramInviteCommandsInGroupAreRefused(t *testing.T) {
	for _, chatType := range []string{"group", "supergroup"} {
		t.Run(chatType, func(t *testing.T) {
			_, pool, fake := tiSetup(t)
			bindChat(t, "group", tgGroupID)
			issueInvite(t, pool, tiOtherPretty)

			for _, cmd := range []string{
				"/invite " + tiPretty,
				"/пригласить " + tiPretty,
				"/invite",
				"/invites",
				"/приглашения",
				"/uninvite " + tiOtherPretty,
				"/отозвать " + tiOtherPretty,
			} {
				requireText(t, ownerSaysInGroup(t, fake, chatType, cmd), tiOnlyPrivate, "в группе "+cmd)
			}

			// Контакт в группе приглашением не становится.
			fake.pushContact(tgOwnerID, tgOwnerNick, tgGroupID, chatType, tiDigits)
			settle(t, fake)

			if _, ok := inviteAttempts(t, pool, tiStored); ok {
				t.Fatal("команда в группе выдала приглашение")
			}
			if _, ok := inviteAttempts(t, pool, tiOtherStored); !ok {
				t.Fatal("/uninvite в группе отозвал приглашение")
			}
			for _, s := range fake.sentTo(tgGroupID) {
				if strings.Contains(s.text, "Код:") || strings.Contains(s.text, tiOtherStored) {
					t.Fatalf("в группу ушло то, что должно оставаться в личке: %s", describeSent([]tgSent{s}))
				}
			}
		})
	}
}

// /invite в группе не заставляет бота ждать номер в личке: следующий
// номер владельца в личку — не приглашение (ФТ-27, ФТ-32).
func TestTelegramInviteInGroupDoesNotWaitInPrivate(t *testing.T) {
	_, pool, fake := tiSetup(t)

	requireText(t, ownerSaysInGroup(t, fake, "supergroup", "/invite"), tiOnlyPrivate, "/invite в группе")
	fake.pushOwnerPrivate(tiPretty)
	settle(t, fake)

	requireNoInvites(t, pool, "номер в личку после /invite в группе")
}

// Не владелец: в личке на команды приглашений — ответ из требования 7
// (текст 019), контакт ничего не выдаёт; в группе — тишина (ФТ-32).
func TestTelegramInviteCommandsFromStranger(t *testing.T) {
	_, pool, fake := tiSetup(t)
	issueInvite(t, pool, tiOtherPretty)

	for i, cmd := range []string{
		"/invite " + tiPretty,
		"/пригласить " + tiPretty,
		"/invite",
		"/invites",
		"/uninvite " + tiOtherPretty,
	} {
		fake.push(tgStrangerID, "tester_vasya", tgStrangerID, "private", cmd)
		got := fake.waitSent(tgStrangerID, i+1, "ответ чужому на "+cmd)
		requireText(t, got[i], tgStrangerText, "чужой в личке: "+cmd)
	}

	// Ожидания номера у чужого нет: его номер следом — не приглашение.
	fake.push(tgStrangerID, "tester_vasya", tgStrangerID, "private", tiPretty)
	fake.pushContact(tgStrangerID, "tester_vasya", tgStrangerID, "private", tiDigits)
	fake.push(tgStrangerID, "tester_vasya", tgGroupID, "supergroup", "/invite "+tiPretty)
	settle(t, fake)

	if _, ok := inviteAttempts(t, pool, tiStored); ok {
		t.Fatal("не владелец получил приглашение")
	}
	if _, ok := inviteAttempts(t, pool, tiOtherStored); !ok {
		t.Fatal("/uninvite не владельца отозвал приглашение")
	}
	if got := fake.sentTo(tgGroupID); len(got) != 0 {
		t.Fatalf("на команду не владельца в группе бот должен молчать, отправлено: %s", describeSent(got))
	}
	requireNoCodeSent(t, fake, "команды не владельца")
}
