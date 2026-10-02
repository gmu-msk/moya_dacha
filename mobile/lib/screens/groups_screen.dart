// Группы: specs/029-groups.md, требования 32–33.
//
// Вкладки «Мои» и «Найти», поиск по названию и описанию, фильтр по типу и
// «Создать» справа вверху. Списки приходят целиком: групп в сообществе
// единицы, сервис отдаёт до ста.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/loading_view.dart';
import '../widgets/place_field.dart';
import '../widgets/segment_tabs.dart';
import '../widgets/user_avatar.dart';
import 'group_screen.dart';
import 'new_group_screen.dart';

/// Через сколько после последней буквы поиска спрашивать сервис.
const _searchDelay = Duration(milliseconds: 400);

/// Фильтр по типу: подпись и значение `kind` (null — все).
const _kinds = <(String, String?)>[
  ('Все', null),
  ('По интересам', 'interest'),
  ('По месту', 'place'),
];

class GroupsScreen extends StatefulWidget {
  const GroupsScreen({super.key, required this.token, required this.viewerId});

  final String token;
  final String viewerId;

  @override
  State<GroupsScreen> createState() => _GroupsScreenState();
}

class _GroupsScreenState extends State<GroupsScreen> {
  final TextEditingController _query = TextEditingController();
  Timer? _debounce;

  /// 0 — «Мои», 1 — «Найти».
  int _tab = 0;
  String? _kind;

  List<Group>? _groups;
  String? _error;

  /// Номер последнего запроса: ответ устаревшего не показываем.
  int _request = 0;

  GroupsApi get _api => GroupsApi(apiClient(token: widget.token));

  @override
  void initState() {
    super.initState();
    usage.screen('groups');
    _load();
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _query.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final request = ++_request;
    setState(() => _error = null);
    final query = _query.text.trim();
    try {
      final list = await _api.getGroups(
        scope: _tab == 0 ? 'mine' : 'available',
        kind: _kind,
        q: query.isEmpty ? null : query,
      );
      debugPrint(
        '$logMarker screen=groups tab=$_tab kind=$_kind '
        'items=${list?.items.length}',
      );
      if (!mounted || request != _request) {
        return;
      }
      setState(() => _groups = list?.items ?? const []);
    } on Exception catch (error) {
      if (!mounted || request != _request) {
        return;
      }
      setState(() => _error = errorMessage(error));
    }
  }

  void _queryChanged(String _) {
    _debounce?.cancel();
    _debounce = Timer(_searchDelay, _load);
  }

  void _select(int tab) {
    if (tab == _tab) {
      return;
    }
    setState(() {
      _tab = tab;
      _groups = null;
    });
    _load();
  }

  void _filter(String? kind) {
    setState(() {
      _kind = kind;
      _groups = null;
    });
    _load();
  }

  Future<void> _create() async {
    final created = await Navigator.of(context).push<Group>(
      MaterialPageRoute(builder: (_) => NewGroupScreen(token: widget.token)),
    );
    if (created == null || !mounted) {
      return;
    }
    // Новая группа — в «Моих», и сразу её экран.
    setState(() {
      _tab = 0;
      _groups = null;
    });
    unawaited(_load());
    await _open(created);
  }

  Future<void> _open(Group group) async {
    await openGroup(
      context,
      token: widget.token,
      viewerId: widget.viewerId,
      groupId: group.id,
      group: group,
    );
    if (mounted) {
      await _load();
    }
  }

  @override
  Widget build(BuildContext context) {
    final groups = _groups;
    final error = _error;
    final searching = _query.text.trim().isNotEmpty || _kind != null;

    final Widget list;
    if (error != null && groups == null) {
      list = Padding(
        padding: const EdgeInsets.all(AppGap.large),
        child: ErrorView(message: error, onRetry: _load),
      );
    } else if (groups == null) {
      list = const LoadingView(label: 'Открываю группы…');
    } else if (groups.isEmpty) {
      list = EmptyView(
        icon: Icons.groups_outlined,
        title: searching
            ? 'Ничего не нашлось'
            : _tab == 0
            ? 'Вы пока ни в одной группе. Выберите пункт в профиле — '
                  'и окажетесь в группе соседей, или найдите группу '
                  'по интересам'
            : 'Групп пока нет. Создайте первую',
        action: _tab == 0 && !searching
            ? OutlinedButton(
                onPressed: () => _select(1),
                child: const Text('Найти группу'),
              )
            : null,
      );
    } else {
      list = RefreshIndicator(
        onRefresh: _load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.only(bottom: AppGap.medium),
          children: [
            for (final group in groups)
              GroupTile(
                key: ValueKey(group.id),
                group: group,
                onTap: () => _open(group),
              ),
          ],
        ),
      );
    }

    return AppScreen(
      title: 'Группы',
      padded: false,
      showServerStatus: false,
      actions: [
        TextButton.icon(
          onPressed: _create,
          icon: const Icon(Icons.add),
          label: const Text('Создать'),
        ),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SegmentTabs(
            labels: const ['Мои', 'Найти'],
            selected: _tab,
            onSelect: _select,
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppGap.medium),
            child: TextField(
              controller: _query,
              onChanged: _queryChanged,
              textInputAction: TextInputAction.search,
              onSubmitted: (_) => _load(),
              decoration: const InputDecoration(
                hintText: 'Название или описание',
                prefixIcon: Icon(Icons.search),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(
              AppGap.medium,
              AppGap.small,
              AppGap.medium,
              AppGap.small,
            ),
            child: Wrap(
              spacing: AppGap.small,
              runSpacing: AppGap.small,
              children: [
                for (final (label, kind) in _kinds)
                  ChoiceChip(
                    label: Text(label),
                    selected: _kind == kind,
                    onSelected: (_) => _filter(kind),
                  ),
              ],
            ),
          ),
          Expanded(child: list),
        ],
      ),
    );
  }
}

/// «По месту · 12 участников» — тип и число участников (требование 33).
String groupSummary(Group group) {
  final kind = group.kind == GroupKindEnum.place ? 'По месту' : 'По интересам';
  return '$kind · ${membersText(group.members)}';
}

/// «1 участник», «3 участника», «12 участников».
String membersText(int n) {
  final last = n % 10;
  final lastTwo = n % 100;
  final word = last == 1 && lastTwo != 11
      ? 'участник'
      : last >= 2 && last <= 4 && (lastTwo < 12 || lastTwo > 14)
      ? 'участника'
      : 'участников';
  return '$n $word';
}

/// Где геогруппа: район и область пункта, а без них — сам пункт.
String groupPlaceText(Group group) {
  final place = group.place;
  if (place == null) {
    return '';
  }
  return place.area.isEmpty ? place.name : place.area;
}

/// Смотрящий — хозяин или участник группы.
bool isGroupMember(Group? group) =>
    group?.membership == GroupMembershipEnum.owner ||
    group?.membership == GroupMembershipEnum.member;

/// Строка группы в списке (требование 33).
class GroupTile extends StatelessWidget {
  const GroupTile({super.key, required this.group, required this.onTap});

  final Group group;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final quiet = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );
    final place = group.place;
    final distance = group.distanceKm;
    final badge = switch (group.membership) {
      GroupMembershipEnum.invited => 'Вас пригласили',
      GroupMembershipEnum.owner => 'Вы создатель',
      _ => group.near ? 'Рядом с вами' : null,
    };

    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppGap.medium,
          vertical: AppGap.snug,
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            CircleAvatar(
              radius: AvatarRadius.inPost,
              backgroundColor: theme.colorScheme.secondaryContainer,
              foregroundColor: theme.colorScheme.onSecondaryContainer,
              child: Icon(
                group.kind == GroupKindEnum.place
                    ? Icons.holiday_village_outlined
                    : Icons.local_florist_outlined,
              ),
            ),
            const SizedBox(width: AppGap.snug),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(group.name, style: theme.textTheme.titleMedium),
                  Text(groupSummary(group), style: quiet),
                  if (place != null)
                    Text(
                      distance == null
                          ? groupPlaceText(group)
                          : '${groupPlaceText(group)} · '
                                '${distanceText(distance)}',
                      style: quiet,
                    ),
                  if (badge != null) ...[
                    const SizedBox(height: AppGap.tiny),
                    Text(
                      badge,
                      style: theme.textTheme.labelLarge?.copyWith(
                        color: theme.colorScheme.secondary,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
