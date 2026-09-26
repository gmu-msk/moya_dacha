//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class FeedbackApi {
  FeedbackApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Мои отзывы
  ///
  /// Свои отзывы разработчику, новые сверху, не больше 50, и что с ними стало (specs/019-feedback.md, требования 17 и 19). 
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> getMyFeedbackWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/feedback';

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

  /// Мои отзывы
  ///
  /// Свои отзывы разработчику, новые сверху, не больше 50, и что с ними стало (specs/019-feedback.md, требования 17 и 19). 
  Future<FeedbackList?> getMyFeedback({ Future<void>? abortTrigger, }) async {
    final response = await getMyFeedbackWithHttpInfo(abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'FeedbackList',) as FeedbackList;
    
    }
    return null;
  }

  /// Написать разработчику
  ///
  /// Отзыв с необязательным скриншотом. Сервис записывает его и в фоне заводит задачу GitHub «входящее» (specs/019-feedback.md, требование 18). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] text (required):
  ///   Текст отзыва, 1–4000 символов после обрезки пробелов
  ///
  /// * [MultipartFile] screenshot:
  ///   Скриншот JPEG или PNG до 10 МБ
  ///
  /// * [String] appVersion:
  ///   Версия приложения, например «1.0.0 (386900)»
  ///
  /// * [String] device:
  ///   Телефон, например «Google Pixel 7, Android 14»
  Future<Response> sendFeedbackWithHttpInfo(String text, { MultipartFile? screenshot, String? appVersion, String? device, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/feedback';

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['multipart/form-data'];

    bool hasFields = false;
    final mp = MultipartRequest('POST', Uri.parse(path));
    if (text != null) {
      hasFields = true;
      mp.fields[r'text'] = parameterToString(text);
    }
    if (screenshot != null) {
      hasFields = true;
      mp.fields[r'screenshot'] = screenshot.field;
      mp.files.add(screenshot);
    }
    if (appVersion != null) {
      hasFields = true;
      mp.fields[r'app_version'] = parameterToString(appVersion);
    }
    if (device != null) {
      hasFields = true;
      mp.fields[r'device'] = parameterToString(device);
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

  /// Написать разработчику
  ///
  /// Отзыв с необязательным скриншотом. Сервис записывает его и в фоне заводит задачу GitHub «входящее» (specs/019-feedback.md, требование 18). 
  ///
  /// Parameters:
  ///
  /// * [String] text (required):
  ///   Текст отзыва, 1–4000 символов после обрезки пробелов
  ///
  /// * [MultipartFile] screenshot:
  ///   Скриншот JPEG или PNG до 10 МБ
  ///
  /// * [String] appVersion:
  ///   Версия приложения, например «1.0.0 (386900)»
  ///
  /// * [String] device:
  ///   Телефон, например «Google Pixel 7, Android 14»
  Future<Feedback?> sendFeedback(String text, { MultipartFile? screenshot, String? appVersion, String? device, Future<void>? abortTrigger, }) async {
    final response = await sendFeedbackWithHttpInfo(text, screenshot: screenshot, appVersion: appVersion, device: device, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Feedback',) as Feedback;
    
    }
    return null;
  }
}
