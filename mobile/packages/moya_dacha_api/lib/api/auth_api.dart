//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class AuthApi {
  AuthApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Войти по коду подтверждения
  ///
  /// Регистрация и вход — одно действие: если номера ещё нет, пользователь создаётся, и вход помечается как первый. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [SessionRequest] sessionRequest (required):
  Future<Response> createSessionWithHttpInfo(SessionRequest sessionRequest, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/auth/session';

    // ignore: prefer_final_locals
    Object? postBody = sessionRequest;

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

  /// Войти по коду подтверждения
  ///
  /// Регистрация и вход — одно действие: если номера ещё нет, пользователь создаётся, и вход помечается как первый. 
  ///
  /// Parameters:
  ///
  /// * [SessionRequest] sessionRequest (required):
  Future<SessionCreated?> createSession(SessionRequest sessionRequest, { Future<void>? abortTrigger, }) async {
    final response = await createSessionWithHttpInfo(sessionRequest, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'SessionCreated',) as SessionCreated;
    
    }
    return null;
  }

  /// Выйти
  ///
  /// Прекращает действие только этого токена. Сессии на других устройствах продолжают работать. 
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> deleteSessionWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/auth/session';

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

  /// Выйти
  ///
  /// Прекращает действие только этого токена. Сессии на других устройствах продолжают работать. 
  Future<void> deleteSession({ Future<void>? abortTrigger, }) async {
    final response = await deleteSessionWithHttpInfo(abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Чья это сессия
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> getSessionWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/auth/session';

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

  /// Чья это сессия
  Future<SessionInfo?> getSession({ Future<void>? abortTrigger, }) async {
    final response = await getSessionWithHttpInfo(abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'SessionInfo',) as SessionInfo;
    
    }
    return null;
  }

  /// Запросить код подтверждения на номер телефона
  ///
  /// Ответ одинаков для нового и для уже зарегистрированного номера: по нему нельзя узнать, есть ли такой пользователь.  Доставка кода спрятана за интерфейсом: пока SMS-провайдер не выбран, код пишется в лог сервиса (specs/001-auth.md).  В режиме приглашений код не отправляется: сервис только проверяет, что на номер есть приглашение (specs/015-invites.md). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [AuthCodeRequest] authCodeRequest (required):
  Future<Response> requestAuthCodeWithHttpInfo(AuthCodeRequest authCodeRequest, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/auth/code';

    // ignore: prefer_final_locals
    Object? postBody = authCodeRequest;

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

  /// Запросить код подтверждения на номер телефона
  ///
  /// Ответ одинаков для нового и для уже зарегистрированного номера: по нему нельзя узнать, есть ли такой пользователь.  Доставка кода спрятана за интерфейсом: пока SMS-провайдер не выбран, код пишется в лог сервиса (specs/001-auth.md).  В режиме приглашений код не отправляется: сервис только проверяет, что на номер есть приглашение (specs/015-invites.md). 
  ///
  /// Parameters:
  ///
  /// * [AuthCodeRequest] authCodeRequest (required):
  Future<AuthCodeAccepted?> requestAuthCode(AuthCodeRequest authCodeRequest, { Future<void>? abortTrigger, }) async {
    final response = await requestAuthCodeWithHttpInfo(authCodeRequest, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'AuthCodeAccepted',) as AuthCodeAccepted;
    
    }
    return null;
  }
}
