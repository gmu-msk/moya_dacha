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

  /// Опубликовать пост
  ///
  /// От одной до четырёх уже загруженных фотографий и необязательная подпись. Поста без медиа не существует (docs/adr/0006-post-is-media.md).  Порядок фотографий в посте — порядок идентификаторов в `media_ids`. 
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
  /// От одной до четырёх уже загруженных фотографий и необязательная подпись. Поста без медиа не существует (docs/adr/0006-post-is-media.md).  Порядок фотографий в посте — порядок идентификаторов в `media_ids`. 
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

  /// Страница ленты
  ///
  /// Все посты всех пользователей, новые сверху: лента одна на всех (CONTEXT.md). Отдаётся страницами — посты и курсор на продолжение.  Курсор непрозрачен: клиент возвращает его как получил и сам не строит. Он указывает на место в порядке ленты, поэтому посты, выложенные между запросами страниц, не сдвигают и не задваивают уже пролистанное (specs/004-feed.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [int] limit:
  ///   Сколько постов вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<Response> getFeedWithHttpInfo({ int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/feed';

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

  /// Страница ленты
  ///
  /// Все посты всех пользователей, новые сверху: лента одна на всех (CONTEXT.md). Отдаётся страницами — посты и курсор на продолжение.  Курсор непрозрачен: клиент возвращает его как получил и сам не строит. Он указывает на место в порядке ленты, поэтому посты, выложенные между запросами страниц, не сдвигают и не задваивают уже пролистанное (specs/004-feed.md). 
  ///
  /// Parameters:
  ///
  /// * [int] limit:
  ///   Сколько постов вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<Feed?> getFeed({ int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    final response = await getFeedWithHttpInfo(limit: limit, cursor: cursor, abortTrigger: abortTrigger,);
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
