//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class UsersApi {
  UsersApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Профиль пользователя
  ///
  /// Кто этот человек: имя, «о себе», аватар, когда он появился и сколько у него постов. Открывается любому вошедшему, свой профиль — той же ручкой. Номера телефона здесь нет и быть не может (CONTEXT.md, specs/009-user-profile.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID) — `author.id` поста или комментария
  Future<Response> getUserWithHttpInfo(String userId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/users/{userId}'
      .replaceAll('{userId}', userId);

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

  /// Профиль пользователя
  ///
  /// Кто этот человек: имя, «о себе», аватар, когда он появился и сколько у него постов. Открывается любому вошедшему, свой профиль — той же ручкой. Номера телефона здесь нет и быть не может (CONTEXT.md, specs/009-user-profile.md). 
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID) — `author.id` поста или комментария
  Future<UserProfile?> getUser(String userId, { Future<void>? abortTrigger, }) async {
    final response = await getUserWithHttpInfo(userId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'UserProfile',) as UserProfile;
    
    }
    return null;
  }

  /// Посты пользователя страницами
  ///
  /// Только посты этого человека, новые сверху, в том же виде и с тем же курсором, что и лента (specs/004-feed.md, specs/009-user-profile.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  ///
  /// * [int] limit:
  ///   Сколько постов вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<Response> getUserPostsWithHttpInfo(String userId, { int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/users/{userId}/posts'
      .replaceAll('{userId}', userId);

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

  /// Посты пользователя страницами
  ///
  /// Только посты этого человека, новые сверху, в том же виде и с тем же курсором, что и лента (specs/004-feed.md, specs/009-user-profile.md). 
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  ///
  /// * [int] limit:
  ///   Сколько постов вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<Feed?> getUserPosts(String userId, { int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    final response = await getUserPostsWithHttpInfo(userId, limit: limit, cursor: cursor, abortTrigger: abortTrigger,);
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
}
