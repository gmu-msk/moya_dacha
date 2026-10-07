//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class PostsApi {
  PostsApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Оставить комментарий
  ///
  /// Комментарии плоские: ответов на комментарий не существует (CONTEXT.md). В ответе — созданный комментарий, а не пост: весь разговор на каждую реплику не отдаётся (specs/006-comments.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [CommentDraft] commentDraft (required):
  Future<Response> addCommentWithHttpInfo(String postId, CommentDraft commentDraft, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/comments'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody = commentDraft;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['application/json'];


    return apiClient.invokeAPI(
      path,
      'POST',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Оставить комментарий
  ///
  /// Комментарии плоские: ответов на комментарий не существует (CONTEXT.md). В ответе — созданный комментарий, а не пост: весь разговор на каждую реплику не отдаётся (specs/006-comments.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [CommentDraft] commentDraft (required):
  Future<Comment?> addComment(String postId, CommentDraft commentDraft, { Future<void>? abortTrigger, }) async {
    final response = await addCommentWithHttpInfo(postId, commentDraft, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Comment',) as Comment;
    
    }
    return null;
  }

  /// Добавить пост в закладки
  ///
  /// Один пользователь — не больше одной закладки на пост. Идемпотентно: повтор ничего не меняет, время закладки остаётся первым. В ответе — пост целиком (specs/032-bookmarks.md, требования 1–3). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Response> bookmarkPostWithHttpInfo(String postId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/bookmark'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'PUT',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Добавить пост в закладки
  ///
  /// Один пользователь — не больше одной закладки на пост. Идемпотентно: повтор ничего не меняет, время закладки остаётся первым. В ответе — пост целиком (specs/032-bookmarks.md, требования 1–3). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Post?> bookmarkPost(String postId, { Future<void>? abortTrigger, }) async {
    final response = await bookmarkPostWithHttpInfo(postId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Post',) as Post;
    
    }
    return null;
  }

  /// Опубликовать пост
  ///
  /// От одной до десяти уже загруженных фотографий и необязательная подпись. Поста без медиа не существует (docs/adr/0006-post-is-media.md).  Порядок фотографий в посте — порядок идентификаторов в `media_ids`.  Тэги поста сервер берёт из хэштегов подписи (specs/028-post-tags.md, требования 3–6). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [PostDraft] postDraft (required):
  Future<Response> createPostWithHttpInfo(PostDraft postDraft, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts';

    // ignore: prefer_final_locals
    Object? postBody = postDraft;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['application/json'];


    return apiClient.invokeAPI(
      path,
      'POST',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Опубликовать пост
  ///
  /// От одной до десяти уже загруженных фотографий и необязательная подпись. Поста без медиа не существует (docs/adr/0006-post-is-media.md).  Порядок фотографий в посте — порядок идентификаторов в `media_ids`.  Тэги поста сервер берёт из хэштегов подписи (specs/028-post-tags.md, требования 3–6). 
  ///
  /// Parameters:
  ///
  /// * [PostDraft] postDraft (required):
  Future<Post?> createPost(PostDraft postDraft, { Future<void>? abortTrigger, }) async {
    final response = await createPostWithHttpInfo(postDraft, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Post',) as Post;
    
    }
    return null;
  }

  /// Удалить свой комментарий
  ///
  /// Комментарий удаляется по адресу своего поста: сначала должен найтись пост, потом комментарий под ним, и только потом проверяется, чей он. Чужой комментарий не удаляется даже автором поста (specs/007-deletion.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [String] commentId (required):
  ///   Идентификатор комментария (UUID)
  Future<Response> deleteCommentWithHttpInfo(String postId, String commentId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/comments/{commentId}'
      .replaceAll('{postId}', postId)
      .replaceAll('{commentId}', commentId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'DELETE',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Удалить свой комментарий
  ///
  /// Комментарий удаляется по адресу своего поста: сначала должен найтись пост, потом комментарий под ним, и только потом проверяется, чей он. Чужой комментарий не удаляется даже автором поста (specs/007-deletion.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [String] commentId (required):
  ///   Идентификатор комментария (UUID)
  Future<void> deleteComment(String postId, String commentId, { Future<void>? abortTrigger, }) async {
    final response = await deleteCommentWithHttpInfo(postId, commentId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Удалить свой пост
  ///
  /// Удаление жёсткое: вместе с постом уходят его фотографии, лайки и комментарии, в том числе чужие (docs/adr/0007-hard-delete.md). Удалить можно только своё; чужое убирает владелец сервиса по жалобе (specs/007-deletion.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Response> deletePostWithHttpInfo(String postId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'DELETE',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Удалить свой пост
  ///
  /// Удаление жёсткое: вместе с постом уходят его фотографии, лайки и комментарии, в том числе чужие (docs/adr/0007-hard-delete.md). Удалить можно только своё; чужое убирает владелец сервиса по жалобе (specs/007-deletion.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<void> deletePost(String postId, { Future<void>? abortTrigger, }) async {
    final response = await deletePostWithHttpInfo(postId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Изменить подпись своего поста
  ///
  /// Меняется только подпись: фотографии, видимость, лайки и комментарии остаются. Проверки как у удаления, потом подпись — как при создании поста (specs/022-edit-block-delete.md). Тэги поста заменяются хэштегами новой подписи (specs/028-post-tags.md, требование 6). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [CaptionUpdate] captionUpdate (required):
  Future<Response> editCaptionWithHttpInfo(String postId, CaptionUpdate captionUpdate, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/caption'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody = captionUpdate;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['application/json'];


    return apiClient.invokeAPI(
      path,
      'PUT',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Изменить подпись своего поста
  ///
  /// Меняется только подпись: фотографии, видимость, лайки и комментарии остаются. Проверки как у удаления, потом подпись — как при создании поста (specs/022-edit-block-delete.md). Тэги поста заменяются хэштегами новой подписи (specs/028-post-tags.md, требование 6). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [CaptionUpdate] captionUpdate (required):
  Future<Post?> editCaption(String postId, CaptionUpdate captionUpdate, { Future<void>? abortTrigger, }) async {
    final response = await editCaptionWithHttpInfo(postId, captionUpdate, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Post',) as Post;
    
    }
    return null;
  }

  /// Изменить свой комментарий
  ///
  /// Проверки как у удаления: пост, комментарий под ним, «своё ли», и только потом текст. Текст проверяется как у нового комментария. Место в разговоре не меняется (specs/022-edit-block-delete.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [String] commentId (required):
  ///   Идентификатор комментария (UUID)
  ///
  /// * [CommentDraft] commentDraft (required):
  Future<Response> editCommentWithHttpInfo(String postId, String commentId, CommentDraft commentDraft, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/comments/{commentId}'
      .replaceAll('{postId}', postId)
      .replaceAll('{commentId}', commentId);

    // ignore: prefer_final_locals
    Object? postBody = commentDraft;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['application/json'];


    return apiClient.invokeAPI(
      path,
      'PUT',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Изменить свой комментарий
  ///
  /// Проверки как у удаления: пост, комментарий под ним, «своё ли», и только потом текст. Текст проверяется как у нового комментария. Место в разговоре не меняется (specs/022-edit-block-delete.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [String] commentId (required):
  ///   Идентификатор комментария (UUID)
  ///
  /// * [CommentDraft] commentDraft (required):
  Future<Comment?> editComment(String postId, String commentId, CommentDraft commentDraft, { Future<void>? abortTrigger, }) async {
    final response = await editCommentWithHttpInfo(postId, commentId, commentDraft, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Comment',) as Comment;
    
    }
    return null;
  }

  /// «Сохранённые» — мои закладки
  ///
  /// Посты из своих закладок страницами, новые сверху по времени закладки. Только посты, видимые смотрящему сейчас (specs/032-bookmarks.md, требования 7 и 8). Курсор непрозрачен. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [int] limit:
  ///   Сколько записей вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<Response> getBookmarksWithHttpInfo({ int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/bookmarks';

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    if (limit != null) {
      queryParams.addAll(_queryParams('', 'limit', limit));
    }
    if (cursor != null) {
      queryParams.addAll(_queryParams('', 'cursor', cursor));
    }

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'GET',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// «Сохранённые» — мои закладки
  ///
  /// Посты из своих закладок страницами, новые сверху по времени закладки. Только посты, видимые смотрящему сейчас (specs/032-bookmarks.md, требования 7 и 8). Курсор непрозрачен. 
  ///
  /// Parameters:
  ///
  /// * [int] limit:
  ///   Сколько записей вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<Feed?> getBookmarks({ int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    final response = await getBookmarksWithHttpInfo(limit: limit, cursor: cursor, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Feed',) as Feed;
    
    }
    return null;
  }

  /// Комментарии поста
  ///
  /// Комментарии поста от старого к новому: это разговор, и читается он сверху вниз. Приходят целиком, без страниц — под постом их столько, что курсор не нужен (specs/006-comments.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Response> getCommentsWithHttpInfo(String postId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/comments'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'GET',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Комментарии поста
  ///
  /// Комментарии поста от старого к новому: это разговор, и читается он сверху вниз. Приходят целиком, без страниц — под постом их столько, что курсор не нужен (specs/006-comments.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Comments?> getComments(String postId, { Future<void>? abortTrigger, }) async {
    final response = await getCommentsWithHttpInfo(postId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Comments',) as Comments;
    
    }
    return null;
  }

  /// Страница ленты
  ///
  /// Посты новые сверху. Во вкладке «Все» — посты всех пользователей, кроме закрытых профилей, на которые смотрящий не подписан; во вкладке «Подписки» — только тех, на кого он подписан, и его собственные (specs/012-follows.md). Отдаётся страницами — посты и курсор на продолжение.  Курсор непрозрачен: клиент возвращает его как получил и сам не строит. Он указывает на место в порядке ленты, поэтому посты, выложенные между запросами страниц, не сдвигают и не задваивают уже пролистанное (specs/004-feed.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] scope:
  ///   Вкладка ленты: `all` — «Все» (по умолчанию), `following` — «Подписки»: посты тех, на кого смотрящий подписан, его собственные (specs/012-follows.md, требования 15–18) и посты групп, где он участник (specs/030-group-posts.md, требование 6). 
  ///
  /// * [int] limit:
  ///   Сколько постов вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  ///
  /// * [String] tag:
  ///   Только посты с этим тэгом; нормализуется, как тэги поста. Пустой — как без параметра (specs/028-post-tags.md, требования 12–14). 
  Future<Response> getFeedWithHttpInfo({ String? scope, int? limit, String? cursor, String? tag, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/feed';

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    if (scope != null) {
      queryParams.addAll(_queryParams('', 'scope', scope));
    }
    if (limit != null) {
      queryParams.addAll(_queryParams('', 'limit', limit));
    }
    if (cursor != null) {
      queryParams.addAll(_queryParams('', 'cursor', cursor));
    }
    if (tag != null) {
      queryParams.addAll(_queryParams('', 'tag', tag));
    }

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'GET',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Страница ленты
  ///
  /// Посты новые сверху. Во вкладке «Все» — посты всех пользователей, кроме закрытых профилей, на которые смотрящий не подписан; во вкладке «Подписки» — только тех, на кого он подписан, и его собственные (specs/012-follows.md). Отдаётся страницами — посты и курсор на продолжение.  Курсор непрозрачен: клиент возвращает его как получил и сам не строит. Он указывает на место в порядке ленты, поэтому посты, выложенные между запросами страниц, не сдвигают и не задваивают уже пролистанное (specs/004-feed.md). 
  ///
  /// Parameters:
  ///
  /// * [String] scope:
  ///   Вкладка ленты: `all` — «Все» (по умолчанию), `following` — «Подписки»: посты тех, на кого смотрящий подписан, его собственные (specs/012-follows.md, требования 15–18) и посты групп, где он участник (specs/030-group-posts.md, требование 6). 
  ///
  /// * [int] limit:
  ///   Сколько постов вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  ///
  /// * [String] tag:
  ///   Только посты с этим тэгом; нормализуется, как тэги поста. Пустой — как без параметра (specs/028-post-tags.md, требования 12–14). 
  Future<Feed?> getFeed({ String? scope, int? limit, String? cursor, String? tag, Future<void>? abortTrigger, }) async {
    final response = await getFeedWithHttpInfo(scope: scope, limit: limit, cursor: cursor, tag: tag, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Feed',) as Feed;
    
    }
    return null;
  }

  /// Показать пост
  ///
  /// Лента одна на всех, поэтому пост открывается любому вошедшему пользователю, а не только автору (CONTEXT.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Response> getPostWithHttpInfo(String postId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'GET',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Показать пост
  ///
  /// Лента одна на всех, поэтому пост открывается любому вошедшему пользователю, а не только автору (CONTEXT.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Post?> getPost(String postId, { Future<void>? abortTrigger, }) async {
    final response = await getPostWithHttpInfo(postId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Post',) as Post;
    
    }
    return null;
  }

  /// Подсказки тэгов для черновика поста
  ///
  /// До пяти тэгов: сначала найденные в подписи, потом популярные у сообщества, потом из стартового словаря. Хэштеги подписи не подсказываются (specs/028-post-tags.md, требования 15–19). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] text:
  ///   Подпись черновика; учитываются первые 1000 знаков
  ///
  /// * [List<String>] exclude:
  ///   Тэги, которые не подсказывать; их в ответе нет
  ///
  /// * [String] prefix:
  ///   Начало тэга, который набирается после `#`; непустой — только тэги, начинающиеся с него 
  Future<Response> getTagSuggestionsWithHttpInfo({ String? text, List<String>? exclude, String? prefix, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/tags/suggestions';

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    if (text != null) {
      queryParams.addAll(_queryParams('', 'text', text));
    }
    if (exclude != null) {
      queryParams.addAll(_queryParams('multi', 'exclude', exclude));
    }
    if (prefix != null) {
      queryParams.addAll(_queryParams('', 'prefix', prefix));
    }

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'GET',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Подсказки тэгов для черновика поста
  ///
  /// До пяти тэгов: сначала найденные в подписи, потом популярные у сообщества, потом из стартового словаря. Хэштеги подписи не подсказываются (specs/028-post-tags.md, требования 15–19). 
  ///
  /// Parameters:
  ///
  /// * [String] text:
  ///   Подпись черновика; учитываются первые 1000 знаков
  ///
  /// * [List<String>] exclude:
  ///   Тэги, которые не подсказывать; их в ответе нет
  ///
  /// * [String] prefix:
  ///   Начало тэга, который набирается после `#`; непустой — только тэги, начинающиеся с него 
  Future<TagSuggestions?> getTagSuggestions({ String? text, List<String>? exclude, String? prefix, Future<void>? abortTrigger, }) async {
    final response = await getTagSuggestionsWithHttpInfo(text: text, exclude: exclude, prefix: prefix, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'TagSuggestions',) as TagSuggestions;
    
    }
    return null;
  }

  /// Поставить лайк
  ///
  /// Один пользователь — не больше одного лайка на пост (CONTEXT.md). Запрос идемпотентен: повторный лайк ничего не меняет и не ошибка.  В ответе — пост целиком, с новым числом лайков: клиенту не нужно досчитывать его самому (specs/005-likes.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Response> likePostWithHttpInfo(String postId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/like'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'PUT',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Поставить лайк
  ///
  /// Один пользователь — не больше одного лайка на пост (CONTEXT.md). Запрос идемпотентен: повторный лайк ничего не меняет и не ошибка.  В ответе — пост целиком, с новым числом лайков: клиенту не нужно досчитывать его самому (specs/005-likes.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Post?> likePost(String postId, { Future<void>? abortTrigger, }) async {
    final response = await likePostWithHttpInfo(postId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Post',) as Post;
    
    }
    return null;
  }

  /// Пожаловаться на чужой комментарий
  ///
  /// Жалоба подаётся по адресу поста, под которым лежит комментарий: пара должна сойтись, иначе `comment_not_found` (specs/008-reports.md). На свой комментарий не жалуются — свой удаляют. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [String] commentId (required):
  ///   Идентификатор комментария (UUID)
  ///
  /// * [ReportDraft] reportDraft:
  Future<Response> reportCommentWithHttpInfo(String postId, String commentId, { ReportDraft? reportDraft, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/comments/{commentId}/report'
      .replaceAll('{postId}', postId)
      .replaceAll('{commentId}', commentId);

    // ignore: prefer_final_locals
    Object? postBody = reportDraft;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['application/json'];


    return apiClient.invokeAPI(
      path,
      'POST',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Пожаловаться на чужой комментарий
  ///
  /// Жалоба подаётся по адресу поста, под которым лежит комментарий: пара должна сойтись, иначе `comment_not_found` (specs/008-reports.md). На свой комментарий не жалуются — свой удаляют. 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [String] commentId (required):
  ///   Идентификатор комментария (UUID)
  ///
  /// * [ReportDraft] reportDraft:
  Future<void> reportComment(String postId, String commentId, { ReportDraft? reportDraft, Future<void>? abortTrigger, }) async {
    final response = await reportCommentWithHttpInfo(postId, commentId, reportDraft: reportDraft, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Пожаловаться на чужой пост
  ///
  /// Жалоба — сигнал владельцу сервиса, который разбирает её вручную. Она ничего не скрывает и ничего не меняет: пост остаётся на месте, автор о жалобе не узнаёт (CONTEXT.md, ADR-0017).  На свой пост не жалуются — свой удаляют (specs/007-deletion.md). Повторная жалоба того же человека принимается так же, как первая, и ничего не меняет (specs/008-reports.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [ReportDraft] reportDraft:
  Future<Response> reportPostWithHttpInfo(String postId, { ReportDraft? reportDraft, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/report'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody = reportDraft;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['application/json'];


    return apiClient.invokeAPI(
      path,
      'POST',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Пожаловаться на чужой пост
  ///
  /// Жалоба — сигнал владельцу сервиса, который разбирает её вручную. Она ничего не скрывает и ничего не меняет: пост остаётся на месте, автор о жалобе не узнаёт (CONTEXT.md, ADR-0017).  На свой пост не жалуются — свой удаляют (specs/007-deletion.md). Повторная жалоба того же человека принимается так же, как первая, и ничего не меняет (specs/008-reports.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [ReportDraft] reportDraft:
  Future<void> reportPost(String postId, { ReportDraft? reportDraft, Future<void>? abortTrigger, }) async {
    final response = await reportPostWithHttpInfo(postId, reportDraft: reportDraft, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Сменить видимость своего поста
  ///
  /// Лайки и комментарии остаются; кто перестал видеть пост, их тоже не видит (specs/013-post-visibility.md, требование 6). Чужой видимый пост — `403 not_your_post`, невидимый — `404`. Видимость группы снимается (specs/031-group-visibility.md, требование 12). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [PostVisibilityUpdate] postVisibilityUpdate (required):
  Future<Response> setPostVisibilityWithHttpInfo(String postId, PostVisibilityUpdate postVisibilityUpdate, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/visibility'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody = postVisibilityUpdate;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['application/json'];


    return apiClient.invokeAPI(
      path,
      'PUT',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Сменить видимость своего поста
  ///
  /// Лайки и комментарии остаются; кто перестал видеть пост, их тоже не видит (specs/013-post-visibility.md, требование 6). Чужой видимый пост — `403 not_your_post`, невидимый — `404`. Видимость группы снимается (specs/031-group-visibility.md, требование 12). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  ///
  /// * [PostVisibilityUpdate] postVisibilityUpdate (required):
  Future<Post?> setPostVisibility(String postId, PostVisibilityUpdate postVisibilityUpdate, { Future<void>? abortTrigger, }) async {
    final response = await setPostVisibilityWithHttpInfo(postId, postVisibilityUpdate, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Post',) as Post;
    
    }
    return null;
  }

  /// Убрать пост из закладок
  ///
  /// Идемпотентно: убрать закладку, которой не было, — не ошибка (specs/032-bookmarks.md, требование 3). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Response> unbookmarkPostWithHttpInfo(String postId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/bookmark'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'DELETE',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Убрать пост из закладок
  ///
  /// Идемпотентно: убрать закладку, которой не было, — не ошибка (specs/032-bookmarks.md, требование 3). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Post?> unbookmarkPost(String postId, { Future<void>? abortTrigger, }) async {
    final response = await unbookmarkPostWithHttpInfo(postId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Post',) as Post;
    
    }
    return null;
  }

  /// Снять лайк
  ///
  /// Идемпотентно: снять лайк, которого не было, — не ошибка (specs/005-likes.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Response> unlikePostWithHttpInfo(String postId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/posts/{postId}/like'
      .replaceAll('{postId}', postId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


    return apiClient.invokeAPI(
      path,
      'DELETE',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Снять лайк
  ///
  /// Идемпотентно: снять лайк, которого не было, — не ошибка (specs/005-likes.md). 
  ///
  /// Parameters:
  ///
  /// * [String] postId (required):
  ///   Идентификатор поста (UUID)
  Future<Post?> unlikePost(String postId, { Future<void>? abortTrigger, }) async {
    final response = await unlikePostWithHttpInfo(postId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Post',) as Post;
    
    }
    return null;
  }

  /// Загрузить фотографию
  ///
  /// Фотография JPEG или PNG не больше 10 МБ. Сервис уменьшает её до 1600 по большей стороне и пересохраняет в JPEG.  Загруженная фотография ещё не опубликована: она ждёт, пока автор соберёт из неё пост (specs/003-posts.md). Каждая фотография идёт своим запросом — на плохой связи переотправить нужно только ту, что не долетела. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [MultipartFile] file (required):
  ///   Фотография JPEG или PNG
  Future<Response> uploadMediaWithHttpInfo(MultipartFile file, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/media';

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['multipart/form-data'];

    bool hasFields = false;
    final mp = MultipartRequest('POST', Uri.parse(path));
    if (file != null) {
      hasFields = true;
      mp.fields[r'file'] = file.field;
      mp.files.add(file);
    }
    if (hasFields) {
      postBody = mp;
    }

    return apiClient.invokeAPI(
      path,
      'POST',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Загрузить фотографию
  ///
  /// Фотография JPEG или PNG не больше 10 МБ. Сервис уменьшает её до 1600 по большей стороне и пересохраняет в JPEG.  Загруженная фотография ещё не опубликована: она ждёт, пока автор соберёт из неё пост (specs/003-posts.md). Каждая фотография идёт своим запросом — на плохой связи переотправить нужно только ту, что не долетела. 
  ///
  /// Parameters:
  ///
  /// * [MultipartFile] file (required):
  ///   Фотография JPEG или PNG
  Future<Media?> uploadMedia(MultipartFile file, { Future<void>? abortTrigger, }) async {
    final response = await uploadMediaWithHttpInfo(file, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Media',) as Media;
    
    }
    return null;
  }
}
