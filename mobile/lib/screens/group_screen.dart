// Экран группы: specs/029-groups.md, требования 32–33.
//
// Название, тип, место, описание, хозяин и участники; кнопка — по
// отношению смотрящего к группе. Хозяину — заявки, приглашённые,
// «Пригласить», «Убрать» и «Удалить группу».
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/confirm.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/loading_view.dart';
import '../widgets/person_row.dart';
import '../widgets/place_field.dart';
import 'groups_screen.dart';
import 'user_screen.dart';

/// Открыть группу поверх текущего экрана. [group] — то, что уже известно
/// из списка: его видно сразу, пока экран спрашивает свежее.
Future<void> openGroup(
  BuildContext context, {
  required String token,
  required String viewerId,
  required String groupId,
  Group? group,
}) => Navigator.of(context).push<void>(
  MaterialPageRoute(
    builder: (_) => GroupScreen(
      token: token,
      viewerId: viewerId,
      groupId: groupId,
      group: group,
    ),
  ),
);

class GroupScreen extends StatefulWidget {
  const GroupScreen({
    super.key,
    required this.token,
    required this.viewerId,
    required this.groupId,
    this.group,
  });

  final String token;
  final String viewerId;
  final String groupId;
  final Group? group;

  @override
  State<GroupScreen> createState() => _GroupScreenState();
}

class _GroupScreenState extends State<GroupScreen> {
  late Group? _group = widget.group;
  List<GroupMember>? _members;
  List<GroupMember> _requested = const [];
  List<GroupMember> _invited = const [];
  String? _error;

  /// Группы больше нет (или она стала не видна).
  bool _gone = false;
  bool _busy = false;

  GroupsApi get _api => GroupsApi(apiClient(token: widget.token));

  bool get _owner => _group?.membership == GroupMembershipEnum.owner;

  @override
  void initState() {
    super.initState();
    usage.screen('group');
    _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final group = await _api.getGroup(widget.groupId);
      final members = await _api.getGroupMembers(widget.groupId);
      var requested = const <GroupMember>[];
      var invited = const <GroupMember>[];
      if (group?.membership == GroupMembershipEnum.owner) {
        requested =
            (await _api.getGroupMembers(
              widget.groupId,
              state: 'requested',
            ))?.items ??
            const [];
        invited =
            (await _api.getGroupMembers(
              widget.groupId,
              state: 'invited',
            ))?.items ??
            const [];
      }
      debugPrint(
        '$logMarker screen=group membership=${group?.membership} '
        'members=${members?.items.length} requested=${requested.length}',
      );
      if (!mounted) {
        return;
      }
      setState(() {
        _group = group;
        _members = members?.items ?? const [];
        _requested = requested;
        _invited = invited;
      });
    } on Exception catch (error) {
      if (!mounted) {
        return;
      }
      setState(() {
        if (serviceErrorCode(error) == 'group_not_found') {
          _gone = true;
        } else {
          _error = errorMessage(error);
        }
      });
    }
  }

  /// Любое действие: ждём сервис, потом перечитываем группу целиком —
  /// меняются и кнопка, и состав, и счётчики.
  Future<void> _act(Future<void> Function() action, String log) async {
    setState(() => _busy = true);
    try {
      await action();
      debugPrint('$logMarker group=$log');
      await _load();
    } on Exception catch (error) {
      debugPrint('$logMarker group=${log}_failed error=$error');
      if (serviceErrorCode(error) == 'group_not_found') {
        if (mounted) {
          setState(() => _gone = true);
        }
      } else {
        _say(errorMessage(error));
      }
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  void _say(String message) {
    if (mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(message)));
    }
  }

  Future<void> _join() => _act(() => _api.joinGroup(widget.groupId), 'joined');

  Future<void> _leave() async {
    final group = _group;
    if (group == null) {
      return;
    }
    if (group.membership == GroupMembershipEnum.member) {
      final agreed = await confirmDelete(
        context,
        title: 'Выйти из группы?',
        question: 'Выйти из группы «${group.name}»?',
        action: 'Выйти',
      );
      if (!agreed || !mounted) {
        return;
      }
    }
    await _act(() => _api.leaveGroup(widget.groupId), 'left');
  }

  Future<void> _accept(GroupMember person) => _act(
    () => _api.addGroupMember(widget.groupId, person.user.id),
    'member_added',
  );

  Future<void> _remove(GroupMember person, {bool ask = false}) async {
    if (ask) {
      final agreed = await confirmDelete(
        context,
        title: 'Убрать из группы?',
        question: 'Убрать ${person.user.nickname} из группы?',
        action: 'Убрать',
      );
      if (!agreed || !mounted) {
        return;
      }
    }
    await _act(
      () => _api.removeGroupMember(widget.groupId, person.user.id),
      'member_removed',
    );
  }

  Future<void> _delete() async {
    final group = _group;
    if (group == null) {
      return;
    }
    final agreed = await confirmDelete(
      context,
      title: 'Удалить группу?',
      question:
          'Удалить группу «${group.name}»? Участники потеряют её, '
          'это не отменить.',
    );
    if (!agreed || !mounted) {
      return;
    }
    setState(() => _busy = true);
    try {
      await _api.deleteGroup(widget.groupId);
      debugPrint('$logMarker group=deleted');
      if (mounted) {
        Navigator.of(context).pop();
      }
    } on Exception catch (error) {
      _say(errorMessage(error));
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  /// Пригласить — выбор из своих подписок и подписчиков (требование 33).
  Future<void> _invite() async {
    final taken = {
      ...?_members?.map((m) => m.user.id),
      ..._invited.map((m) => m.user.id),
    };
    final person = await showModalBottomSheet<FollowUser>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (_) => _InvitePicker(
        token: widget.token,
        viewerId: widget.viewerId,
        exclude: taken,
      ),
    );
    if (person == null || !mounted) {
      return;
    }
    await _act(() => _api.addGroupMember(widget.groupId, person.id), 'invited');
  }

  void _openPerson(String userId) => openUserProfile(
    context,
    token: widget.token,
    viewerId: widget.viewerId,
    userId: userId,
  );

  @override
  Widget build(BuildContext context) {
    final group = _group;
    final error = _error;

    final Widget body;
    if (_gone) {
      body = const EmptyView(
        icon: Icons.groups_outlined,
        title: 'Группы больше нет',
      );
    } else if (group == null && error != null) {
      body = Padding(
        padding: const EdgeInsets.all(AppGap.large),
        child: ErrorView(message: error, onRetry: _load),
      );
    } else if (group == null) {
      body = const LoadingView(label: 'Открываю группу…');
    } else {
      body = RefreshIndicator(
        onRefresh: _load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.only(bottom: AppGap.large),
          children: [
            _header(group),
            if (_owner) ..._ownerSections(),
            _SectionTitle('Участники · ${group.members}'),
            ..._memberRows(group),
            if (error != null)
              Padding(
                padding: const EdgeInsets.all(AppGap.medium),
                child: ErrorView(message: error, onRetry: _load),
              ),
            if (_owner)
              Padding(
                padding: const EdgeInsets.fromLTRB(
                  AppGap.medium,
                  AppGap.large,
                  AppGap.medium,
                  0,
                ),
                child: TextButton(
                  onPressed: _busy ? null : _delete,
                  style: TextButton.styleFrom(
                    foregroundColor: Theme.of(context).colorScheme.error,
                  ),
                  child: const Text('Удалить группу'),
                ),
              ),
          ],
        ),
      );
    }

    return AppScreen(
      title: group?.name ?? 'Группа',
      padded: false,
      showServerStatus: false,
      child: body,
    );
  }

  Widget _header(Group group) {
    final theme = Theme.of(context);
    final quiet = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );
    final place = group.place;
    final distance = group.distanceKm;
    final policy = switch (group.joinPolicy) {
      GroupJoinPolicyEnum.open => 'Открытая',
      GroupJoinPolicyEnum.request => 'По заявке',
      GroupJoinPolicyEnum.invite => 'По приглашению',
    };

    return Padding(
      padding: const EdgeInsets.all(AppGap.medium),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(group.name, style: theme.textTheme.headlineSmall),
          const SizedBox(height: AppGap.tiny),
          Text('${groupSummary(group)} · $policy', style: quiet),
          if (place != null) ...[
            const SizedBox(height: AppGap.small),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Padding(
                  padding: const EdgeInsets.only(right: AppGap.tiny),
                  child: Icon(
                    Icons.place_outlined,
                    size: 18,
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                Expanded(
                  child: Text(
                    distance == null
                        ? groupPlaceText(group)
                        : '${groupPlaceText(group)} · '
                              '${distanceText(distance)}',
                    style: quiet,
                  ),
                ),
              ],
            ),
            if (group.near)
              Padding(
                padding: const EdgeInsets.only(top: AppGap.tiny),
                child: Text(
                  'Рядом с вами',
                  style: theme.textTheme.labelLarge?.copyWith(
                    color: theme.colorScheme.secondary,
                  ),
                ),
              ),
          ],
          if (group.description.isNotEmpty) ...[
            const SizedBox(height: AppGap.medium),
            Text(group.description, style: theme.textTheme.bodyLarge),
          ],
          const SizedBox(height: AppGap.small),
          Text('Создатель — ${group.owner.nickname}', style: quiet),
          const SizedBox(height: AppGap.medium),
          _membershipButtons(group),
        ],
      ),
    );
  }

  /// Кнопка по отношению смотрящего (требование 32).
  Widget _membershipButtons(Group group) {
    final busy = _busy;
    return switch (group.membership) {
      GroupMembershipEnum.owner => FilledButton.icon(
        onPressed: busy ? null : _invite,
        icon: const Icon(Icons.person_add_alt_outlined),
        label: const Text('Пригласить'),
      ),
      GroupMembershipEnum.member => OutlinedButton(
        onPressed: busy ? null : _leave,
        child: const Text('Выйти'),
      ),
      GroupMembershipEnum.requested => OutlinedButton(
        onPressed: busy ? null : _leave,
        child: const Text('Заявка отправлена'),
      ),
      GroupMembershipEnum.invited => Wrap(
        spacing: AppGap.small,
        runSpacing: AppGap.small,
        children: [
          FilledButton(
            onPressed: busy ? null : _join,
            child: const Text('Принять приглашение'),
          ),
          OutlinedButton(
            onPressed: busy ? null : _leave,
            child: const Text('Отклонить'),
          ),
        ],
      ),
      _ => FilledButton(
        onPressed: busy ? null : _join,
        child: Text(
          group.joinPolicy == GroupJoinPolicyEnum.request
              ? 'Попроситься'
              : 'Вступить',
        ),
      ),
    };
  }

  /// Заявки и приглашённые — только хозяину (требование 33).
  List<Widget> _ownerSections() => [
    if (_requested.isNotEmpty) ...[
      const _SectionTitle('Заявки'),
      for (final person in _requested)
        PersonRow(
          key: ValueKey('requested-${person.user.id}'),
          nickname: person.user.nickname,
          name: person.user.name,
          avatarUrl: person.user.avatarUrl,
          onTap: () => _openPerson(person.user.id),
          trailing: Wrap(
            spacing: AppGap.tiny,
            children: [
              IconButton.filled(
                tooltip: 'Принять',
                onPressed: _busy ? null : () => _accept(person),
                icon: const Icon(Icons.check),
              ),
              IconButton.outlined(
                tooltip: 'Отклонить',
                onPressed: _busy ? null : () => _remove(person),
                icon: const Icon(Icons.close),
              ),
            ],
          ),
        ),
    ],
    if (_invited.isNotEmpty) ...[
      const _SectionTitle('Приглашены'),
      for (final person in _invited)
        PersonRow(
          key: ValueKey('invited-${person.user.id}'),
          nickname: person.user.nickname,
          name: person.user.name,
          avatarUrl: person.user.avatarUrl,
          onTap: () => _openPerson(person.user.id),
          trailing: TextButton(
            onPressed: _busy ? null : () => _remove(person),
            child: const Text('Отозвать'),
          ),
        ),
    ],
  ];

  List<Widget> _memberRows(Group group) {
    final members = _members;
    if (members == null) {
      return const [
        Padding(
          padding: EdgeInsets.all(AppGap.medium),
          child: Center(child: CircularProgressIndicator()),
        ),
      ];
    }
    return [
      for (final person in members)
        PersonRow(
          key: ValueKey('member-${person.user.id}'),
          nickname: person.user.nickname,
          name: person.user.name,
          avatarUrl: person.user.avatarUrl,
          subtitle: person.role == GroupMemberRoleEnum.owner
              ? 'Создатель группы'
              : null,
          onTap: () => _openPerson(person.user.id),
          trailing: _owner && person.role != GroupMemberRoleEnum.owner
              ? TextButton(
                  onPressed: _busy ? null : () => _remove(person, ask: true),
                  child: const Text('Убрать'),
                )
              : null,
        ),
      if (members.length <= 1)
        Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppGap.medium,
            vertical: AppGap.small,
          ),
          child: Text(
            'Пока только создатель группы',
            style: Theme.of(context).textTheme.bodyMedium,
          ),
        ),
    ];
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(
      AppGap.medium,
      AppGap.medium,
      AppGap.medium,
      AppGap.tiny,
    ),
    child: Text(text, style: Theme.of(context).textTheme.titleSmall),
  );
}

/// Выбор, кого пригласить: свои подписки и подписчики без тех, кто уже в
/// группе или приглашён. Списки небольшие — первые страницы по 50.
class _InvitePicker extends StatefulWidget {
  const _InvitePicker({
    required this.token,
    required this.viewerId,
    required this.exclude,
  });

  final String token;
  final String viewerId;
  final Set<String> exclude;

  @override
  State<_InvitePicker> createState() => _InvitePickerState();
}

class _InvitePickerState extends State<_InvitePicker> {
  List<FollowUser>? _people;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final api = FollowsApi(apiClient(token: widget.token));
      final following = await api.getFollowing(widget.viewerId, limit: 50);
      final followers = await api.getFollowers(widget.viewerId, limit: 50);
      final seen = <String>{...widget.exclude, widget.viewerId};
      final people = <FollowUser>[
        for (final person in [...?following?.items, ...?followers?.items])
          if (seen.add(person.id)) person,
      ];
      if (mounted) {
        setState(() => _people = people);
      }
    } on Exception catch (error) {
      if (mounted) {
        setState(() => _error = errorMessage(error));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final people = _people;
    final error = _error;
    final Widget body;
    if (error != null) {
      body = Padding(
        padding: const EdgeInsets.all(AppGap.large),
        child: ErrorView(message: error, onRetry: _load),
      );
    } else if (people == null) {
      body = const Padding(
        padding: EdgeInsets.all(AppGap.large),
        child: Center(child: CircularProgressIndicator()),
      );
    } else if (people.isEmpty) {
      body = const Padding(
        padding: EdgeInsets.all(AppGap.large),
        child: EmptyView(
          icon: Icons.person_add_alt_outlined,
          title: 'Пригласить некого',
          hint: 'Здесь ваши подписки и подписчики, которых ещё нет в группе',
        ),
      );
    } else {
      body = ListView(
        shrinkWrap: true,
        children: [
          for (final person in people)
            PersonRow(
              nickname: person.nickname,
              name: person.name,
              avatarUrl: person.avatarUrl,
              onTap: () => Navigator.of(context).pop(person),
            ),
        ],
      );
    }
    return SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
            child: Text(
              'Пригласить в группу',
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ),
          Flexible(child: body),
        ],
      ),
    );
  }
}
