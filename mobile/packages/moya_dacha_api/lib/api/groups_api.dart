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

  /// Пригласить в группу
  ///
  /// Приглашает любой участник. Участник и приглашённый не меняются (specs/029-groups.md, требование 23). 
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

  /// Пригласить в группу
  ///
  /// Приглашает любой участник. Участник и приглашённый не меняются (specs/029-groups.md, требование 23). 
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

  /// Создать группу по интересам
  ///
  /// Создатель становится хозяином и первым участником. Геогруппы создаются сами (specs/029-groups.md, требование 10). 
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

  /// Создать группу по интересам
  ///
  /// Создатель становится хозяином и первым участником. Геогруппы создаются сами (specs/029-groups.md, требование 10). 
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

  /// Удалить свою группу по интересам
  ///
  /// Вместе с составом и приглашениями. Геогруппу удалить нельзя (specs/029-groups.md, требование 11). 
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

  /// Удалить свою группу по интересам
  ///
  /// Вместе с составом и приглашениями. Геогруппу удалить нельзя (specs/029-groups.md, требование 11). 
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

  /// Состав группы или приглашения
  ///
  /// Без `state` — участники, хозяин первым, дальше новые выше; видят все, кому видна группа. `invited` — только участникам, новые сверху; `requested` — всегда пусто, заявок больше нет (specs/029-groups.md, требования 21–22). 
  ///
  /// Note: This method returns the HTTP [Response].
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  ///
  /// * [String] state:
  ///   `member` (по умолчанию) или `invited`; `requested` — всегда пусто
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

  /// Состав группы или приглашения
  ///
  /// Без `state` — участники, хозяин первым, дальше новые выше; видят все, кому видна группа. `invited` — только участникам, новые сверху; `requested` — всегда пусто, заявок больше нет (specs/029-groups.md, требования 21–22). 
  ///
  /// Parameters:
  ///
  /// * [String] groupId (required):
  ///   Идентификатор группы (UUID)
  ///
  /// * [String] state:
  ///   `member` (по умолчанию) или `invited`; `requested` — всегда пусто
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

  /// Ждущие приглашения мне в группы
  ///
  /// Новые сверху, `user` — кто пригласил (specs/029-groups.md, требования 26–27). Это не события раздела «Уведомления», а список над ними, как заявки на подписку. 
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

  /// Ждущие приглашения мне в группы
  ///
  /// Новые сверху, `user` — кто пригласил (specs/029-groups.md, требования 26–27). Это не события раздела «Уведомления», а список над ними, как заявки на подписку. 
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
  /// До 100 групп (specs/029-groups.md, требования 19–20). `mine` — где смотрящий хозяин или участник: геогруппы первыми, внутри по названию; `available` — остальные видимые ему: приглашения, потом геогруппа его пункта, потом больше участников выше. 
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
  /// До 100 групп (specs/029-groups.md, требования 19–20). `mine` — где смотрящий хозяин или участник: геогруппы первыми, внутри по названию; `available` — остальные видимые ему: приглашения, потом геогруппа его пункта, потом больше участников выше. 
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

  /// Вступить в группу или принять приглашение
  ///
  /// Все группы открытые: смотрящий сразу участник (specs/029-groups.md, требование 17). Повтор ничего не меняет. 
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

  /// Вступить в группу или принять приглашение
  ///
  /// Все группы открытые: смотрящий сразу участник (specs/029-groups.md, требование 17). Повтор ничего не меняет. 
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

  /// Выйти или отклонить приглашение
  ///
  /// Строка смотрящего удаляется; без строки — не ошибка (specs/029-groups.md, требование 18). 
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

  /// Выйти или отклонить приглашение
  ///
  /// Строка смотрящего удаляется; без строки — не ошибка (specs/029-groups.md, требование 18). 
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

  /// Отозвать приглашение или убрать участника
  ///
  /// Только хозяин группы по интересам; без строки — не ошибка (specs/029-groups.md, требование 24). 
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

  /// Отозвать приглашение или убрать участника
  ///
  /// Только хозяин группы по интересам; без строки — не ошибка (specs/029-groups.md, требование 24). 
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
