//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class UsageApi {
  UsageApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Конец сессии в приложении
  ///
  /// Приложение ушло с экрана. Сервис ставит время конца «сейчас» и заменяет экраны сессии присланными. Повторная отметка сдвигает конец (specs/020-app-sessions.md, требования 5–9). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] sessionId (required):
  ///
  /// * [AppSessionEnd] appSessionEnd:
  Future<Response> endAppSessionWithHttpInfo(String sessionId, { AppSessionEnd? appSessionEnd, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/app-sessions/{sessionId}/end'
      .replaceAll('{sessionId}', sessionId);

    // ignore: prefer_final_locals
    Object? postBody = appSessionEnd;

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

  /// Конец сессии в приложении
  ///
  /// Приложение ушло с экрана. Сервис ставит время конца «сейчас» и заменяет экраны сессии присланными. Повторная отметка сдвигает конец (specs/020-app-sessions.md, требования 5–9). 
  ///
  /// Parameters:
  ///
  /// * [String] sessionId (required):
  ///
  /// * [AppSessionEnd] appSessionEnd:
  Future<void> endAppSession(String sessionId, { AppSessionEnd? appSessionEnd, Future<void>? abortTrigger, }) async {
    final response = await endAppSessionWithHttpInfo(sessionId, appSessionEnd: appSessionEnd, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Отчёт о необработанной ошибке приложения
  ///
  /// Приложение поймало ошибку, которую никто не обработал, и молча сообщает о ней. Сервис склеивает повторы одной ошибки в группу (specs/021-app-errors.md, требования 1–7). Токен не обязателен: без него или с недействительным отчёт записывается как отчёт без входа, `401` ручка не отвечает. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [AppErrorReport] appErrorReport (required):
  Future<Response> reportAppErrorWithHttpInfo(AppErrorReport appErrorReport, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/app-errors';

    // ignore: prefer_final_locals
    Object? postBody = appErrorReport;

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

  /// Отчёт о необработанной ошибке приложения
  ///
  /// Приложение поймало ошибку, которую никто не обработал, и молча сообщает о ней. Сервис склеивает повторы одной ошибки в группу (specs/021-app-errors.md, требования 1–7). Токен не обязателен: без него или с недействительным отчёт записывается как отчёт без входа, `401` ручка не отвечает. 
  ///
  /// Parameters:
  ///
  /// * [AppErrorReport] appErrorReport (required):
  Future<void> reportAppError(AppErrorReport appErrorReport, { Future<void>? abortTrigger, }) async {
    final response = await reportAppErrorWithHttpInfo(appErrorReport, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Начало сессии в приложении
  ///
  /// Приложение вышло на экран у вошедшего человека. Сервис заводит сессию со временем начала по своим часам (specs/020-app-sessions.md, требования 1–4). 
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> startAppSessionWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/app-sessions';

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>[];


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

  /// Начало сессии в приложении
  ///
  /// Приложение вышло на экран у вошедшего человека. Сервис заводит сессию со временем начала по своим часам (specs/020-app-sessions.md, требования 1–4). 
  Future<AppSessionStarted?> startAppSession({ Future<void>? abortTrigger, }) async {
    final response = await startAppSessionWithHttpInfo(abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'AppSessionStarted',) as AppSessionStarted;
    
    }
    return null;
  }
}
