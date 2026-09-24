//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class FollowsApi {
  FollowsApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Принять заявку
  ///
  /// Заявитель становится подписчиком (specs/012-follows.md, требование 10).
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<Response> acceptFollowRequestWithHttpInfo(String userId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/follow-requests/{userId}'
      .replaceAll('{userId}', userId);

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

  /// Принять заявку
  ///
  /// Заявитель становится подписчиком (specs/012-follows.md, требование 10).
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<void> acceptFollowRequest(String userId, { Future<void>? abortTrigger, }) async {
    final response = await acceptFollowRequestWithHttpInfo(userId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Отклонить заявку
  ///
  /// Заявка исчезает молча: заявитель снова видит «Подписаться» (specs/012-follows.md, требование 10). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<Response> declineFollowRequestWithHttpInfo(String userId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/follow-requests/{userId}'
      .replaceAll('{userId}', userId);

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

  /// Отклонить заявку
  ///
  /// Заявка исчезает молча: заявитель снова видит «Подписаться» (specs/012-follows.md, требование 10). 
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<void> declineFollowRequest(String userId, { Future<void>? abortTrigger, }) async {
    final response = await declineFollowRequestWithHttpInfo(userId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Подписаться или подать заявку
  ///
  /// На открытый профиль подписка оформляется сразу, на закрытый — подаётся заявка (specs/012-follows.md, требование 2). Идемпотентно: повторная подписка или заявка ничего не меняет и не ошибка. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<Response> followUserWithHttpInfo(String userId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/users/{userId}/follow'
      .replaceAll('{userId}', userId);

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

  /// Подписаться или подать заявку
  ///
  /// На открытый профиль подписка оформляется сразу, на закрытый — подаётся заявка (specs/012-follows.md, требование 2). Идемпотентно: повторная подписка или заявка ничего не меняет и не ошибка. 
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<Relation?> followUser(String userId, { Future<void>? abortTrigger, }) async {
    final response = await followUserWithHttpInfo(userId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Relation',) as Relation;
    
    }
    return null;
  }

  /// Заявки на подписку ко мне
  ///
  /// Ждущие заявки, новые сверху (specs/012-follows.md).
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
  Future<Response> getFollowRequestsWithHttpInfo({ int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/follow-requests';

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

  /// Заявки на подписку ко мне
  ///
  /// Ждущие заявки, новые сверху (specs/012-follows.md).
  ///
  /// Parameters:
  ///
  /// * [int] limit:
  ///   Сколько записей вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<AuthorList?> getFollowRequests({ int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    final response = await getFollowRequestsWithHttpInfo(limit: limit, cursor: cursor, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'AuthorList',) as AuthorList;
    
    }
    return null;
  }

  /// Подписчики пользователя страницами
  ///
  /// Кто подписан на пользователя, новые связи сверху. Заявки сюда не входят. У закрытого профиля список видят только хозяин и подписчики (specs/012-follows.md, требования 7 и 14). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  ///
  /// * [int] limit:
  ///   Сколько записей вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<Response> getFollowersWithHttpInfo(String userId, { int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/users/{userId}/followers'
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

  /// Подписчики пользователя страницами
  ///
  /// Кто подписан на пользователя, новые связи сверху. Заявки сюда не входят. У закрытого профиля список видят только хозяин и подписчики (specs/012-follows.md, требования 7 и 14). 
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  ///
  /// * [int] limit:
  ///   Сколько записей вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<FollowList?> getFollowers(String userId, { int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    final response = await getFollowersWithHttpInfo(userId, limit: limit, cursor: cursor, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'FollowList',) as FollowList;
    
    }
    return null;
  }

  /// Подписки пользователя страницами
  ///
  /// На кого подписан пользователь, новые связи сверху. Заявки сюда не входят. У закрытого профиля список видят только хозяин и подписчики (specs/012-follows.md, требования 7 и 14). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  ///
  /// * [int] limit:
  ///   Сколько записей вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<Response> getFollowingWithHttpInfo(String userId, { int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/users/{userId}/following'
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

  /// Подписки пользователя страницами
  ///
  /// На кого подписан пользователь, новые связи сверху. Заявки сюда не входят. У закрытого профиля список видят только хозяин и подписчики (specs/012-follows.md, требования 7 и 14). 
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  ///
  /// * [int] limit:
  ///   Сколько записей вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<FollowList?> getFollowing(String userId, { int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    final response = await getFollowingWithHttpInfo(userId, limit: limit, cursor: cursor, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'FollowList',) as FollowList;
    
    }
    return null;
  }

  /// Закрыть или открыть свой профиль
  ///
  /// Закрытие не трогает тех, кто уже подписан. Открытие принимает все ждущие заявки (specs/012-follows.md, требования 8 и 9). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [PrivacyUpdate] privacyUpdate (required):
  Future<Response> setPrivacyWithHttpInfo(PrivacyUpdate privacyUpdate, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/privacy';

    // ignore: prefer_final_locals
    Object? postBody = privacyUpdate;

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

  /// Закрыть или открыть свой профиль
  ///
  /// Закрытие не трогает тех, кто уже подписан. Открытие принимает все ждущие заявки (specs/012-follows.md, требования 8 и 9). 
  ///
  /// Parameters:
  ///
  /// * [PrivacyUpdate] privacyUpdate (required):
  Future<CurrentUser?> setPrivacy(PrivacyUpdate privacyUpdate, { Future<void>? abortTrigger, }) async {
    final response = await setPrivacyWithHttpInfo(privacyUpdate, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'CurrentUser',) as CurrentUser;
    
    }
    return null;
  }

  /// Отписаться или отменить заявку
  ///
  /// Идемпотентно: отписка без подписки — не ошибка (specs/012-follows.md, требование 3). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<Response> unfollowUserWithHttpInfo(String userId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/users/{userId}/follow'
      .replaceAll('{userId}', userId);

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

  /// Отписаться или отменить заявку
  ///
  /// Идемпотентно: отписка без подписки — не ошибка (specs/012-follows.md, требование 3). 
  ///
  /// Parameters:
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<Relation?> unfollowUser(String userId, { Future<void>? abortTrigger, }) async {
    final response = await unfollowUserWithHttpInfo(userId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Relation',) as Relation;
    
    }
    return null;
  }
}
