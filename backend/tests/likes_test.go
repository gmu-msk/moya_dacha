package tests

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

// Третий номер: в лайках нужен не только тот, кто лайкнул, но и тот, кто
// просто смотрит — у него `liked` обязан быть false (specs/005-likes.md,
// «Ограничения и edge cases»).
const thirdPhonePretty = "+7 (900) 222-33-44"

// --- Представления из контракта -------------------------------------------

// likedPostPayload — пост с двумя новыми полями (schema Post, поля `likes`
// и `liked`). Отдельный тип, а не расширенный postPayload: остальные фичи
// про лайки не знают, а здесь важно, что поля приходят вместе с постом.
type likedPostPayload struct {
	postPayload
	Likes int  `json:"likes"`
	Liked bool `json:"liked"`
}

// likedFeedPayload — страница ленты, разобранная с лайками: поля приходят
// везде, где приходит пост (ФТ-4).
type likedFeedPayload struct {
	Items      []likedPostPayload `json:"items"`
	NextCursor *string            `json:"next_cursor"`
}

// --- Хелперы --------------------------------------------------------------

// likePost ставит посту лайк.
func likePost(t *testing.T, baseURL, token, postID string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/posts/"+postID+"/like", token, nil)
}

// unlikePost снимает лайк с поста.
func unlikePost(t *testing.T, baseURL, token, postID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, baseURL+"/posts/"+postID+"/like", token, nil)
}

// likedOK требует ожидаемого статуса и возвращает пост с лайками.
func likedOK(t *testing.T, resp *http.Response, wantStatus int) likedPostPayload {
	t.Helper()

	if resp.StatusCode != wantStatus {
		t.Fatalf("ожидался статус %d, получен %d", wantStatus, resp.StatusCode)
	}

	var body likedPostPayload
	decode(t, resp, &body)

	return body
}

// postToLike публикует пост с одной фотографией: лайкам всё равно, что на
// ней, важен только сам пост.
func postToLike(t *testing.T, baseURL, token string) likedPostPayload {
	t.Helper()

	photo := photoOf(t, baseURL, token, 60, 40)

	return likedOK(t, createPostOf(t, baseURL, token, photo.ID), http.StatusCreated)
}

// likedPostPage открывает пост по его адресу и возвращает его с лайками.
func likedPostPage(t *testing.T, baseURL, token, postID string) likedPostPayload {
	t.Helper()
	return likedOK(t, fetchPost(t, baseURL, token, postID), http.StatusOK)
}

// likedFeedItem находит пост в ленте и возвращает его: лайки приходят
// у каждого поста ленты, и считаются они для того, кто спрашивает (ФТ-4).
func likedFeedItem(t *testing.T, baseURL, token, postID string) likedPostPayload {
	t.Helper()

	resp := fetchFeed(t, baseURL, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на страницу ленты ожидался статус 200, получен %d", resp.StatusCode)
	}

	var page likedFeedPayload
	decode(t, resp, &page)

	for _, item := range page.Items {
		if item.ID == postID {
			return item
		}
	}

	t.Fatalf("поста %s нет в ленте, а он опубликован", postID)

	return likedPostPayload{}
}

// requireLikes требует у поста ровно этого числа лайков и ровно такого
// признака «я отметил».
func requireLikes(t *testing.T, post likedPostPayload, wantLikes int, wantLiked bool, where string) {
	t.Helper()

	if post.Likes != wantLikes {
		t.Errorf("%s: ожидалось лайков %d, получено %d", where, wantLikes, post.Likes)
	}
	if post.Liked != wantLiked {
		t.Errorf("%s: ожидалось liked=%t, получено %t", where, wantLiked, post.Liked)
	}
}

// requireLikesEverywhere требует одних и тех же лайков всюду, где приходит
// пост: на экране поста и в ленте (ФТ-4).
func requireLikesEverywhere(t *testing.T, baseURL, token, postID string, wantLikes int, wantLiked bool, who string) {
	t.Helper()

	requireLikes(t, likedPostPage(t, baseURL, token, postID), wantLikes, wantLiked, who+": пост по своему адресу")
	requireLikes(t, likedFeedItem(t, baseURL, token, postID), wantLikes, wantLiked, who+": пост в ленте")
}

// requireLikeFields требует, чтобы оба поля были в ответе на самом деле:
// пост без лайков приходит с "likes": 0 и "liked": false, а не без полей
// («Пост в ответе»).
func requireLikeFields(t *testing.T, raw []byte, where string) {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON: %v", where, err)
	}

	for _, field := range []string{"likes", "liked"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("%s: в посте нет поля %q, а оно обязательное", where, field)
		}
	}
}

// --- PUT /api/posts/{postId}/like: поставить лайк -------------------------

// Лайк посту, у которого лайков не было: 200, число стало единицей,
// и «я отметил» — правда («Лайк посту без лайков»).
func TestLikePostWithoutLikesMakesItOneAndMine(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	post := postToLike(t, baseURL, token)
	requireLikes(t, post, 0, false, "только что опубликованный пост")

	liked := likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)

	requireLikes(t, liked, 1, true, "ответ на лайк")
	if liked.ID != post.ID {
		t.Errorf("в ответе на лайк ожидался пост %s, получен %s", post.ID, liked.ID)
	}
	requireLikesEverywhere(t, baseURL, token, post.ID, 1, true, "лайкнувший")
}

// Поставить лайк дважды — то же, что поставить один раз: число не растёт,
// ошибки нет («Лайк тому же посту второй раз», ФТ-3).
func TestLikeOnTheSamePostTwiceChangesNothing(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToLike(t, baseURL, token)

	first := likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)
	requireLikes(t, first, 1, true, "первый лайк")

	// На дачной связи повтор запроса — обычное дело, и второй раз он не
	// должен ломаться.
	second := likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)
	requireLikes(t, second, 1, true, "повторный лайк")

	third := likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)
	requireLikes(t, third, 1, true, "третий лайк подряд")

	requireLikesEverywhere(t, baseURL, token, post.ID, 1, true, "лайкнувший дважды")
}

// Лайкнуть можно и свой пост: лента одна на всех, запрета нет
// («Лайк своему посту», ФТ-2).
func TestLikeOwnPostIsAllowed(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	post := postToLike(t, baseURL, token)

	liked := likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)

	requireLikes(t, liked, 1, true, "лайк своему посту")
	if liked.Author.ID != userID {
		t.Errorf("пост должен остаться своим: ожидался автор %s, получен %s", userID, liked.Author.ID)
	}
}

// Лайк чужому посту ставится так же, как своему («Лайк чужому посту»).
func TestLikeAnotherUsersPostIsAllowed(t *testing.T) {
	baseURL := startAPI(t)

	authorToken, authorID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, authorToken, profileName)
	readerToken, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToLike(t, baseURL, authorToken)

	liked := likedOK(t, likePost(t, baseURL, readerToken, post.ID), http.StatusOK)

	requireLikes(t, liked, 1, true, "лайк чужому посту")
	if liked.Author.ID != authorID {
		t.Errorf("автором поста остаётся %s, а в ответе %s", authorID, liked.Author.ID)
	}

	// Автор видит, что его пост заметили, но сам он не отмечал.
	requireLikesEverywhere(t, baseURL, authorToken, post.ID, 1, false, "автор")
	requireLikesEverywhere(t, baseURL, readerToken, post.ID, 1, true, "читатель")
}

// Два пользователя лайкнули один пост: число — два, и у каждого из них
// «я отметил» — правда («Два пользователя лайкнули один пост»).
func TestTwoUsersLikeOnePostAndBothAreCounted(t *testing.T) {
	baseURL := startAPI(t)

	authorToken, _ := signIn(t, baseURL, phonePretty)
	secondToken, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToLike(t, baseURL, authorToken)

	first := likedOK(t, likePost(t, baseURL, authorToken, post.ID), http.StatusOK)
	requireLikes(t, first, 1, true, "лайк автора")

	second := likedOK(t, likePost(t, baseURL, secondToken, post.ID), http.StatusOK)
	requireLikes(t, second, 2, true, "лайк второго пользователя")

	requireLikesEverywhere(t, baseURL, authorToken, post.ID, 2, true, "первый лайкнувший")
	requireLikesEverywhere(t, baseURL, secondToken, post.ID, 2, true, "второй лайкнувший")
}

// Тот, кто не лайкал, видит число чужих лайков, но у него самого
// `liked` — false, сколько бы лайков ни было («liked у того, кто не
// лайкал», ФТ-5: автор видит число, а не имена).
func TestLikedIsFalseForSomeoneWhoDidNotLike(t *testing.T) {
	baseURL := startAPI(t)

	authorToken, _ := signIn(t, baseURL, phonePretty)
	secondToken, _ := signIn(t, baseURL, otherPhonePretty)
	thirdToken, _ := signIn(t, baseURL, thirdPhonePretty)

	post := postToLike(t, baseURL, authorToken)

	likedOK(t, likePost(t, baseURL, authorToken, post.ID), http.StatusOK)
	likedOK(t, likePost(t, baseURL, secondToken, post.ID), http.StatusOK)

	requireLikesEverywhere(t, baseURL, thirdToken, post.ID, 2, false, "не лайкавший")
}

// --- DELETE /api/posts/{postId}/like: снять лайк --------------------------

// Снять свой лайк: число уменьшается, «я отметил» становится ложью
// («Снять свой лайк»).
func TestUnlikeRemovesOwnLikeAndLowersTheCount(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToLike(t, baseURL, token)

	likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)
	likedOK(t, likePost(t, baseURL, otherToken, post.ID), http.StatusOK)

	unliked := likedOK(t, unlikePost(t, baseURL, token, post.ID), http.StatusOK)

	// Снялся ровно свой лайк: чужой остался на месте.
	requireLikes(t, unliked, 1, false, "ответ на снятие лайка")
	requireLikesEverywhere(t, baseURL, token, post.ID, 1, false, "снявший лайк")
	requireLikesEverywhere(t, baseURL, otherToken, post.ID, 1, true, "не снимавший лайк")
}

// Снять лайк, которого не было, — не ошибка: 200 и ничего не изменилось
// («Снять лайк, которого не было», ФТ-3).
func TestUnlikeWithoutLikeChangesNothingAndIsNotAnError(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)

	post := postToLike(t, baseURL, token)
	likedOK(t, likePost(t, baseURL, otherToken, post.ID), http.StatusOK)

	unliked := likedOK(t, unlikePost(t, baseURL, token, post.ID), http.StatusOK)

	requireLikes(t, unliked, 1, false, "снятие лайка, которого не было")
	requireLikesEverywhere(t, baseURL, otherToken, post.ID, 1, true, "чужой лайк на месте")
}

// Снять лайк дважды — то же, что снять один раз: повтор запроса
// не ломается (ФТ-3).
func TestUnlikeTwiceRemovesTheLikeOnlyOnce(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToLike(t, baseURL, token)

	likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)

	first := likedOK(t, unlikePost(t, baseURL, token, post.ID), http.StatusOK)
	requireLikes(t, first, 0, false, "первое снятие")

	second := likedOK(t, unlikePost(t, baseURL, token, post.ID), http.StatusOK)
	requireLikes(t, second, 0, false, "повторное снятие")

	requireLikesEverywhere(t, baseURL, token, post.ID, 0, false, "снявший лайк дважды")
}

// Сердечко нажимается туда и обратно сколько угодно раз: каждый ответ
// говорит, что стало в итоге (пользовательский сценарий, шаги 2-3).
func TestLikeAndUnlikeCanBeRepeatedBackAndForth(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToLike(t, baseURL, token)

	for round := 1; round <= 3; round++ {
		on := likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)
		requireLikes(t, on, 1, true, "лайк поставлен")

		off := likedOK(t, unlikePost(t, baseURL, token, post.ID), http.StatusOK)
		requireLikes(t, off, 0, false, "лайк снят")
	}
}

// --- Поля поста: везде, где приходит пост ---------------------------------

// Пост без лайков приходит с "likes": 0 и "liked": false, а не без полей,
// и приходит так везде: в ответе на публикацию, на своём экране и в ленте
// («Пост без лайков в ленте», «Пост в ответе»).
func TestPostWithoutLikesCarriesZeroAndFalseEverywhere(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	photo := photoOf(t, baseURL, token, 60, 40)

	created := createPostOf(t, baseURL, token, photo.ID)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("на публикацию ожидался статус 201, получен %d", created.StatusCode)
	}
	createdRaw := rawJSON(t, created)
	requireLikeFields(t, createdRaw, "ответ на публикацию")

	var post likedPostPayload
	if err := json.Unmarshal(createdRaw, &post); err != nil {
		t.Fatalf("ответ на публикацию не разобрался как пост: %v", err)
	}
	requireLikes(t, post, 0, false, "ответ на публикацию")

	requireLikeFields(t, rawJSON(t, fetchPost(t, baseURL, token, post.ID)), "пост по своему адресу")
	requireLikesEverywhere(t, baseURL, token, post.ID, 0, false, "пост без лайков")
}

// Ответ на лайк — пост целиком, ровно такой же, как по его адресу:
// у клиента одно представление поста и один способ его обновить (ФТ-8).
func TestLikeAndUnlikeAnswerWithTheWholePostJustLikeItsOwnPage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	photo := photoOf(t, baseURL, token, 400, 300)
	post := likedOK(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": []string{photo.ID}, "caption": postCaption}), http.StatusCreated)

	requests := []struct {
		name string
		send func() *http.Response
	}{
		{"ответ на лайк", func() *http.Response { return likePost(t, baseURL, token, post.ID) }},
		{"ответ на снятие лайка", func() *http.Response { return unlikePost(t, baseURL, token, post.ID) }},
	}

	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			resp := request.send()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: ожидался статус 200, получен %d", request.name, resp.StatusCode)
			}

			var fromLike map[string]any
			if err := json.Unmarshal(rawJSON(t, resp), &fromLike); err != nil {
				t.Fatalf("%s не разобрался как JSON: %v", request.name, err)
			}

			var fromPage map[string]any
			if err := json.Unmarshal(rawJSON(t, fetchPost(t, baseURL, token, post.ID)), &fromPage); err != nil {
				t.Fatalf("пост по своему адресу не разобрался как JSON: %v", err)
			}

			if !reflect.DeepEqual(fromLike, fromPage) {
				t.Errorf("%s отличается от поста по своему адресу:\nв ответе: %v\nпо адресу: %v",
					request.name, fromLike, fromPage)
			}
		})
	}
}

// Лайки приходят у каждого поста ленты, и `liked` считается для того, кто
// спрашивает: в одной и той же ленте у одного человека сердечки закрашены,
// у другого — нет («Лайки в ленте»).
func TestFeedShowsLikesOfEveryPostForTheOneWhoAsks(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, token, profileName)

	// Три поста: без лайков, с одним лайком соседа и с лайками обоих.
	untouched := postToLike(t, baseURL, token)
	byNeighbour := postToLike(t, baseURL, token)
	byBoth := postToLike(t, baseURL, token)

	likedOK(t, likePost(t, baseURL, otherToken, byNeighbour.ID), http.StatusOK)
	likedOK(t, likePost(t, baseURL, otherToken, byBoth.ID), http.StatusOK)
	likedOK(t, likePost(t, baseURL, token, byBoth.ID), http.StatusOK)

	resp := fetchFeed(t, baseURL, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на страницу ленты ожидался статус 200, получен %d", resp.StatusCode)
	}

	var page likedFeedPayload
	decode(t, resp, &page)

	if len(page.Items) != 3 {
		t.Fatalf("в ленте ожидались три поста, получено %d", len(page.Items))
	}

	// Порядок ленты — только время, лайки его не меняют: новые сверху
	// (specs/004-feed.md, ФТ-1).
	wantOrder := []string{byBoth.ID, byNeighbour.ID, untouched.ID}
	if got := postIDs(toPosts(page.Items)); !reflect.DeepEqual(got, wantOrder) {
		t.Errorf("лайки не меняют порядок ленты: ожидались посты %v, получены %v", wantOrder, got)
	}

	expected := map[string]struct {
		likes int
		liked bool
	}{
		untouched.ID:   {0, false},
		byNeighbour.ID: {1, false},
		byBoth.ID:      {2, true},
	}

	for _, item := range page.Items {
		want, ok := expected[item.ID]
		if !ok {
			t.Fatalf("в ленте оказался посторонний пост %s", item.ID)
		}
		requireLikes(t, item, want.likes, want.liked, "лента того, кто лайкнул один пост")
	}

	// У соседа та же лента, но свои сердечки.
	requireLikes(t, likedFeedItem(t, baseURL, otherToken, untouched.ID), 0, false, "лента соседа, пост без лайков")
	requireLikes(t, likedFeedItem(t, baseURL, otherToken, byNeighbour.ID), 1, true, "лента соседа, его лайк")
	requireLikes(t, likedFeedItem(t, baseURL, otherToken, byBoth.ID), 2, true, "лента соседа, лайк обоих")
}

// toPosts отдаёт посты ленты без полей лайков: так их можно сверить
// хелперами ленты, которые про лайки не знают.
func toPosts(items []likedPostPayload) []postPayload {
	posts := make([]postPayload, 0, len(items))
	for _, item := range items {
		posts = append(posts, item.postPayload)
	}

	return posts
}

// --- Ошибки и доступ ------------------------------------------------------

// Лайк и снятие лайка поста, которого нет: 404 post_not_found. Не похожий
// на UUID идентификатор — тот же ответ: поста с таким идентификатором нет
// («Лайк несуществующему посту», «Идентификатор поста не похож на UUID»,
// «Снять лайк у несуществующего поста»).
func TestLikeAndUnlikeOfUnknownPostAreNotFound(t *testing.T) {
	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 "не-идентификатор",
	}

	requests := map[string]func(t *testing.T, baseURL, token, postID string) *http.Response{
		"PUT /api/posts/{postId}/like":    likePost,
		"DELETE /api/posts/{postId}/like": unlikePost,
	}

	for name, request := range requests {
		for caseName, id := range ids {
			t.Run(name+": "+caseName, func(t *testing.T) {
				baseURL := startAPI(t)

				token, _ := signIn(t, baseURL, phonePretty)

				requireError(t, request(t, baseURL, token, id), http.StatusNotFound, "post_not_found")
			})
		}
	}
}

// Лайк живёт вместе с постом: у поста, который убрали, лайков нет —
// его самого уже нет (ФТ-6, ON DELETE CASCADE).
func TestLikeOfRemovedPostIsNotFound(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToLike(t, baseURL, token)

	likedOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK)

	removePostFromDatabase(t, post.ID)

	requireError(t, likePost(t, baseURL, token, post.ID), http.StatusNotFound, "post_not_found")
	requireError(t, unlikePost(t, baseURL, token, post.ID), http.StatusNotFound, "post_not_found")
}

// Без токена лайков нет, как и всего остального: 401 unauthorized.
// Недействительный токен — то же самое («Любой из запросов без токена»,
// ФТ-10).
func TestLikeRequestsRequireValidToken(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToLike(t, baseURL, token)

	requests := map[string]func(t *testing.T, baseURL, token, postID string) *http.Response{
		"PUT /api/posts/{postId}/like":    likePost,
		"DELETE /api/posts/{postId}/like": unlikePost,
	}

	tokens := map[string]string{
		"без токена":        "",
		"неизвестный токен": "этого-токена-сервис-не-выдавал",
	}

	for name, request := range requests {
		for caseName, badToken := range tokens {
			t.Run(name+": "+caseName, func(t *testing.T) {
				requireError(t, request(t, baseURL, badToken, post.ID), http.StatusUnauthorized, "unauthorized")
			})
		}
	}

	// Лайк никому не поставился: лента для чужого запроса не изменилась.
	requireLikesEverywhere(t, baseURL, token, post.ID, 0, false, "после отвергнутых запросов")
}

// Лайки не переживают выход из сессии чужими руками: после signOut токен
// недействителен, и сердечко им не нажать (specs/001-auth.md).
func TestLikeIsUnavailableAfterSignOut(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToLike(t, baseURL, token)

	if resp := signOut(t, baseURL, token); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на выход ожидался статус 204, получен %d", resp.StatusCode)
	}

	requireError(t, likePost(t, baseURL, token, post.ID), http.StatusUnauthorized, "unauthorized")
	requireError(t, unlikePost(t, baseURL, token, post.ID), http.StatusUnauthorized, "unauthorized")
}
