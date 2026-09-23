//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class ProfileApi {
  ProfileApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Убрать аватар
  ///
  /// Убрать аватар, которого нет, — не ошибка.
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> deleteAvatarWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/avatar';

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

  /// Убрать аватар
  ///
  /// Убрать аватар, которого нет, — не ошибка.
  Future<CurrentUser?> deleteAvatar({ Future<void>? abortTrigger, }) async {
    final response = await deleteAvatarWithHttpInfo(abortTrigger: abortTrigger,);
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

  /// Мой профиль
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> getMeWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me';

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

  /// Мой профиль
  Future<CurrentUser?> getMe({ Future<void>? abortTrigger, }) async {
    final response = await getMeWithHttpInfo(abortTrigger: abortTrigger,);
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

  /// Поставить аватар
  ///
  /// Картинка JPEG или PNG не больше 5 МБ. Сервис уменьшает её до 512×512 и пересохраняет в JPEG; новый аватар заменяет прежний. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [MultipartFile] file (required):
  ///   Картинка JPEG или PNG
  Future<Response> setAvatarWithHttpInfo(MultipartFile file, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/avatar';

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    const contentTypes = <String>['multipart/form-data'];

    bool hasFields = false;
    final mp = MultipartRequest('PUT', Uri.parse(path));
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
      'PUT',
      queryParams,
      postBody,
      headerParams,
      formParams,
      contentTypes.isEmpty ? null : contentTypes.first,
      abortTrigger: abortTrigger,
    );
  }

  /// Поставить аватар
  ///
  /// Картинка JPEG или PNG не больше 5 МБ. Сервис уменьшает её до 512×512 и пересохраняет в JPEG; новый аватар заменяет прежний. 
  ///
  /// Parameters:
  ///
  /// * [MultipartFile] file (required):
  ///   Картинка JPEG или PNG
  Future<CurrentUser?> setAvatar(MultipartFile file, { Future<void>? abortTrigger, }) async {
    final response = await setAvatarWithHttpInfo(file, abortTrigger: abortTrigger,);
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

  /// Выбрать или сменить никнейм
  ///
  /// Никнейм уникален без учёта регистра и хранится так, как его ввели. После успешной смены `nickname_chosen` становится `true`. Свой же никнейм в другом регистре — не ошибка. См. specs/010-nicknames.md. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [NicknameUpdate] nicknameUpdate (required):
  Future<Response> setNicknameWithHttpInfo(NicknameUpdate nicknameUpdate, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/nickname';

    // ignore: prefer_final_locals
    Object? postBody = nicknameUpdate;

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

  /// Выбрать или сменить никнейм
  ///
  /// Никнейм уникален без учёта регистра и хранится так, как его ввели. После успешной смены `nickname_chosen` становится `true`. Свой же никнейм в другом регистре — не ошибка. См. specs/010-nicknames.md. 
  ///
  /// Parameters:
  ///
  /// * [NicknameUpdate] nicknameUpdate (required):
  Future<CurrentUser?> setNickname(NicknameUpdate nicknameUpdate, { Future<void>? abortTrigger, }) async {
    final response = await setNicknameWithHttpInfo(nicknameUpdate, abortTrigger: abortTrigger,);
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

  /// Изменить полное имя и «о себе»
  ///
  /// Замена обоих полей сразу, а не частичное изменение: пустое `about` или `name` очищает поле. Никнейм меняется отдельно — `PUT /me/nickname`. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [ProfileUpdate] profileUpdate (required):
  Future<Response> updateMeWithHttpInfo(ProfileUpdate profileUpdate, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me';

    // ignore: prefer_final_locals
    Object? postBody = profileUpdate;

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

  /// Изменить полное имя и «о себе»
  ///
  /// Замена обоих полей сразу, а не частичное изменение: пустое `about` или `name` очищает поле. Никнейм меняется отдельно — `PUT /me/nickname`. 
  ///
  /// Parameters:
  ///
  /// * [ProfileUpdate] profileUpdate (required):
  Future<CurrentUser?> updateMe(ProfileUpdate profileUpdate, { Future<void>? abortTrigger, }) async {
    final response = await updateMeWithHttpInfo(profileUpdate, abortTrigger: abortTrigger,);
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
}
