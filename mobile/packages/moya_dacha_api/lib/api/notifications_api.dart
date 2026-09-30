//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class NotificationsApi {
  NotificationsApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Мои уведомления
  ///
  /// События страницами, новые сверху; лайки одного поста слиты в одну строку (specs/014-notifications.md). Заявки на подписку сюда не входят — они в `GET /me/follow-requests`. Чтение списка ничего не отмечает прочитанным: для этого `PUT /me/notifications/seen`. 
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
  Future<Response> getNotificationsWithHttpInfo({ int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/notifications';

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

  /// Мои уведомления
  ///
  /// События страницами, новые сверху; лайки одного поста слиты в одну строку (specs/014-notifications.md). Заявки на подписку сюда не входят — они в `GET /me/follow-requests`. Чтение списка ничего не отмечает прочитанным: для этого `PUT /me/notifications/seen`. 
  ///
  /// Parameters:
  ///
  /// * [int] limit:
  ///   Сколько записей вернуть, от 1 до 50
  ///
  /// * [String] cursor:
  ///   Курсор из предыдущего ответа; без него — первая страница
  Future<NotificationList?> getNotifications({ int? limit, String? cursor, Future<void>? abortTrigger, }) async {
    final response = await getNotificationsWithHttpInfo(limit: limit, cursor: cursor, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'NotificationList',) as NotificationList;
    
    }
    return null;
  }

  /// Есть ли новое
  ///
  /// Сколько строк раздела новее последнего открытия и сколько ждущих заявок: по ним горит точка на колокольчике (specs/014-notifications.md, требование 5). 
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> getUnreadNotificationsWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/notifications/unread';

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

  /// Есть ли новое
  ///
  /// Сколько строк раздела новее последнего открытия и сколько ждущих заявок: по ним горит точка на колокольчике (specs/014-notifications.md, требование 5). 
  Future<UnreadNotifications?> getUnreadNotifications({ Future<void>? abortTrigger, }) async {
    final response = await getUnreadNotificationsWithHttpInfo(abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'UnreadNotifications',) as UnreadNotifications;
    
    }
    return null;
  }

  /// Отметить всё прочитанным
  ///
  /// Всё, что пришло до этой минуты, становится прочитанным (specs/014-notifications.md, требование 7). Заявки остаются непрочитанными, пока ждут ответа. 
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> markNotificationsSeenWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/notifications/seen';

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

  /// Отметить всё прочитанным
  ///
  /// Всё, что пришло до этой минуты, становится прочитанным (specs/014-notifications.md, требование 7). Заявки остаются непрочитанными, пока ждут ответа. 
  Future<void> markNotificationsSeen({ Future<void>? abortTrigger, }) async {
    final response = await markNotificationsSeenWithHttpInfo(abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Токен FCM этого телефона
  ///
  /// Телефон отдаёт токен Firebase Cloud Messaging, на него сервис шлёт пуши о новых уведомлениях и заявках (specs/024-push.md). Токен привязан к сессии: у сессии один токен, новый заменяет старый, тот же токен с другой сессии переезжает к ней. Выход из аккаунта забывает токен. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [PushToken] pushToken (required):
  Future<Response> setPushTokenWithHttpInfo(PushToken pushToken, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/push-token';

    // ignore: prefer_final_locals
    Object? postBody = pushToken;

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

  /// Токен FCM этого телефона
  ///
  /// Телефон отдаёт токен Firebase Cloud Messaging, на него сервис шлёт пуши о новых уведомлениях и заявках (specs/024-push.md). Токен привязан к сессии: у сессии один токен, новый заменяет старый, тот же токен с другой сессии переезжает к ней. Выход из аккаунта забывает токен. 
  ///
  /// Parameters:
  ///
  /// * [PushToken] pushToken (required):
  Future<void> setPushToken(PushToken pushToken, { Future<void>? abortTrigger, }) async {
    final response = await setPushTokenWithHttpInfo(pushToken, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }
}
