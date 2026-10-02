//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;


class GroupsApi {
  GroupsApi([ApiClient? apiClient]) : apiClient = apiClient ?? defaultApiClient;

  final ApiClient apiClient;

  /// Принять заявку или пригласить
  ///
  /// Есть заявка — человек становится участником, нет ничего — приглашение. Участник и приглашённый не меняются (specs/029-groups.md, требование 22). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<Response> addGroupMemberWithHttpInfo(String groupId, String userId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups/{groupId}/members/{userId}'
      .replaceAll('{groupId}', groupId)
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

  /// Принять заявку или пригласить
  ///
  /// Есть заявка — человек становится участником, нет ничего — приглашение. Участник и приглашённый не меняются (specs/029-groups.md, требование 22). 
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<GroupMember?> addGroupMember(String groupId, String userId, { Future<void>? abortTrigger, }) async {
    final response = await addGroupMemberWithHttpInfo(groupId, userId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'GroupMember',) as GroupMember;
    
    }
    return null;
  }

  /// Создать группу
  ///
  /// Создатель становится хозяином и первым участником (specs/029-groups.md, требования 1–6). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [GroupDraft] groupDraft (required):
  Future<Response> createGroupWithHttpInfo(GroupDraft groupDraft, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups';

    // ignore: prefer_final_locals
    Object? postBody = groupDraft;

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

  /// Создать группу
  ///
  /// Создатель становится хозяином и первым участником (specs/029-groups.md, требования 1–6). 
  ///
  /// Parameters:
  ///
  /// * [GroupDraft] groupDraft (required):
  Future<Group?> createGroup(GroupDraft groupDraft, { Future<void>? abortTrigger, }) async {
    final response = await createGroupWithHttpInfo(groupDraft, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Group',) as Group;
    
    }
    return null;
  }

  /// Удалить свою группу
  ///
  /// Вместе с составом, заявками и приглашениями (specs/029-groups.md, требование 7). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  Future<Response> deleteGroupWithHttpInfo(String groupId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups/{groupId}'
      .replaceAll('{groupId}', groupId);

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

  /// Удалить свою группу
  ///
  /// Вместе с составом, заявками и приглашениями (specs/029-groups.md, требование 7). 
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  Future<void> deleteGroup(String groupId, { Future<void>? abortTrigger, }) async {
    final response = await deleteGroupWithHttpInfo(groupId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Группа
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  Future<Response> getGroupWithHttpInfo(String groupId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups/{groupId}'
      .replaceAll('{groupId}', groupId);

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

  /// Группа
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  Future<Group?> getGroup(String groupId, { Future<void>? abortTrigger, }) async {
    final response = await getGroupWithHttpInfo(groupId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Group',) as Group;
    
    }
    return null;
  }

  /// Состав группы, заявки или приглашения
  ///
  /// Без `state` — участники, хозяин первым, дальше новые выше; видят все, кому видна группа. `requested` и `invited` — только хозяину, новые сверху (specs/029-groups.md, требования 20–21). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  ///
  /// * [String] state:
  ///   `member` (по умолчанию), `requested` или `invited`
  Future<Response> getGroupMembersWithHttpInfo(String groupId, { String? state, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups/{groupId}/members'
      .replaceAll('{groupId}', groupId);

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    if (state != null) {
      queryParams.addAll(_queryParams('', 'state', state));
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

  /// Состав группы, заявки или приглашения
  ///
  /// Без `state` — участники, хозяин первым, дальше новые выше; видят все, кому видна группа. `requested` и `invited` — только хозяину, новые сверху (specs/029-groups.md, требования 20–21). 
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  ///
  /// * [String] state:
  ///   `member` (по умолчанию), `requested` или `invited`
  Future<GroupMemberList?> getGroupMembers(String groupId, { String? state, Future<void>? abortTrigger, }) async {
    final response = await getGroupMembersWithHttpInfo(groupId, state: state, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'GroupMemberList',) as GroupMemberList;
    
    }
    return null;
  }

  /// Ждущие заявки в мои группы и приглашения мне
  ///
  /// Новые сверху (specs/029-groups.md, требования 25–26). Это не события раздела «Уведомления», а список над ними, как заявки на подписку. 
  ///
  /// Note: This method returns the HTTP [Response].
  Future<Response> getGroupRequestsWithHttpInfo({ Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/me/group-requests';

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

  /// Ждущие заявки в мои группы и приглашения мне
  ///
  /// Новые сверху (specs/029-groups.md, требования 25–26). Это не события раздела «Уведомления», а список над ними, как заявки на подписку. 
  Future<GroupRequestList?> getGroupRequests({ Future<void>? abortTrigger, }) async {
    final response = await getGroupRequestsWithHttpInfo(abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'GroupRequestList',) as GroupRequestList;
    
    }
    return null;
  }

  /// Мои группы или группы, куда можно вступить
  ///
  /// До 100 групп (specs/029-groups.md, требования 16–17). `mine` — где смотрящий хозяин или участник, по названию; `available` — остальные видимые ему: приглашения, потом группы рядом, потом больше участников выше. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] scope:
  ///   `mine` (по умолчанию) или `available`
  ///
  /// * [String] kind:
  ///   `interest` или `place` — только группы этого типа
  ///
  /// * [String] q:
  ///   Поиск по названию и описанию без учёта регистра
  Future<Response> getGroupsWithHttpInfo({ String? scope, String? kind, String? q, Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups';

    // ignore: prefer_final_locals
    Object? postBody;

    final queryParams = <QueryParam>[];
    final headerParams = <String, String>{};
    final formParams = <String, String>{};

    if (scope != null) {
      queryParams.addAll(_queryParams('', 'scope', scope));
    }
    if (kind != null) {
      queryParams.addAll(_queryParams('', 'kind', kind));
    }
    if (q != null) {
      queryParams.addAll(_queryParams('', 'q', q));
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

  /// Мои группы или группы, куда можно вступить
  ///
  /// До 100 групп (specs/029-groups.md, требования 16–17). `mine` — где смотрящий хозяин или участник, по названию; `available` — остальные видимые ему: приглашения, потом группы рядом, потом больше участников выше. 
  ///
  /// Parameters:
  ///
  /// * [String] scope:
  ///   `mine` (по умолчанию) или `available`
  ///
  /// * [String] kind:
  ///   `interest` или `place` — только группы этого типа
  ///
  /// * [String] q:
  ///   Поиск по названию и описанию без учёта регистра
  Future<GroupList?> getGroups({ String? scope, String? kind, String? q, Future<void>? abortTrigger, }) async {
    final response = await getGroupsWithHttpInfo(scope: scope, kind: kind, q: q, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'GroupList',) as GroupList;
    
    }
    return null;
  }

  /// Вступить в группу или попроситься
  ///
  /// Открытая — сразу участник, по заявке — заявка, приглашённый — участник в группе любого правила (specs/029-groups.md, требование 18). Повтор ничего не меняет. Группы по приглашению без приглашения для смотрящего нет — `404`. 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  Future<Response> joinGroupWithHttpInfo(String groupId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups/{groupId}/membership'
      .replaceAll('{groupId}', groupId);

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

  /// Вступить в группу или попроситься
  ///
  /// Открытая — сразу участник, по заявке — заявка, приглашённый — участник в группе любого правила (specs/029-groups.md, требование 18). Повтор ничего не меняет. Группы по приглашению без приглашения для смотрящего нет — `404`. 
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  Future<Group?> joinGroup(String groupId, { Future<void>? abortTrigger, }) async {
    final response = await joinGroupWithHttpInfo(groupId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
    // When a remote server returns no body with a status of 204, we shall not decode it.
    // At the time of writing this, `dart:convert` will throw an "Unexpected end of input"
    // FormatException when trying to decode an empty string.
    if (response.body.isNotEmpty && response.statusCode != HttpStatus.noContent) {
      return await apiClient.deserializeAsync(await _decodeBodyBytes(response), 'Group',) as Group;
    
    }
    return null;
  }

  /// Выйти, отозвать заявку или отклонить приглашение
  ///
  /// Строка смотрящего удаляется; без строки — не ошибка (specs/029-groups.md, требование 19). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  Future<Response> leaveGroupWithHttpInfo(String groupId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups/{groupId}/membership'
      .replaceAll('{groupId}', groupId);

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

  /// Выйти, отозвать заявку или отклонить приглашение
  ///
  /// Строка смотрящего удаляется; без строки — не ошибка (specs/029-groups.md, требование 19). 
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  Future<void> leaveGroup(String groupId, { Future<void>? abortTrigger, }) async {
    final response = await leaveGroupWithHttpInfo(groupId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }

  /// Отклонить заявку, отозвать приглашение или убрать участника
  ///
  /// Без строки — не ошибка (specs/029-groups.md, требование 23). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<Response> removeGroupMemberWithHttpInfo(String groupId, String userId, { Future<void>? abortTrigger, }) async {
    // ignore: prefer_const_declarations
    final path = r'/groups/{groupId}/members/{userId}'
      .replaceAll('{groupId}', groupId)
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

  /// Отклонить заявку, отозвать приглашение или убрать участника
  ///
  /// Без строки — не ошибка (specs/029-groups.md, требование 23). 
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  ///
  /// * [String] userId (required):
  ///   Идентификатор пользователя (UUID)
  Future<void> removeGroupMember(String groupId, String userId, { Future<void>? abortTrigger, }) async {
    final response = await removeGroupMemberWithHttpInfo(groupId, userId, abortTrigger: abortTrigger,);
    if (response.statusCode >= HttpStatus.badRequest) {
      throw ApiException(response.statusCode, await _decodeBodyBytes(response));
    }
  }
}
