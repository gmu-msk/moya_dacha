// Подписчики и подписки человека: specs/012-follows.md, требование 14.
//
// Две вкладки, как в ленте: «N подписчиков» и «M подписок». В каждой
// строке — кнопка подписки, чтобы подписаться на соседа, не открывая
// его профиль. Сам смотрящий приходит без кнопки: «Это вы».
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/author_line.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/follow_button.dart';
import '../widgets/loading_view.dart';
import '../widgets/person_row.dart';
import '../widgets/segment_tabs.dart';
import 'user_screen.dart';

/// Сколько людей запрашивается за раз.
const _pageSize = 30;

enum FollowListTab { followers, following }

class FollowListScreen extends StatefulWidget {
  const FollowListScreen({
    super.key,
    required this.token,
    required this.viewerId,
    required this.user,
    required this.tab,
    this.onFollowChanged,
  });

  final String token;
  final String viewerId;

  /// Чьи это списки.
  final UserProfile user;

  /// Какая вкладка открыта первой.
  final FollowListTab tab;

  /// Смотрящий на кого-то подписался или отписался.
  final VoidCallback? onFollowChanged;

  @override
  State<FollowListScreen> createState() => _FollowListScreenState();
}

class _FollowListScreenState extends State<FollowListScreen> {
  late FollowListTab _tab = widget.tab;

  @override
  Widget build(BuildContext context) {
    final user = widget.user;

    return AppScreen(
      title: user.nickname,
      showServerStatus: false,
      padded: false,
      child: Column(
        children: [
          SegmentTabs(
            labels: [
              countWord(
                user.followers,
                'подписчик',
                'подписчика',
                'подписчиков',
              ),
              countWord(user.following, 'подписка', 'подписки', 'подписок'),
            ],
            selected: _tab.index,
            onSelect: (index) =>
                setState(() => _tab = FollowListTab.values[index]),
          ),
          Expanded(
            child: IndexedStack(
              index: _tab.index,
              children: [
                for (final tab in FollowListTab.values)
                  _PeopleView(
                    token: widget.token,
                    viewerId: widget.viewerId,
                    userId: user.id,
                    tab: tab,
                    onFollowChanged: widget.onFollowChanged,
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Одна вкладка: люди страницами, новые связи сверху.
class _PeopleView extends StatefulWidget {
  const _PeopleView({
    required this.token,
    required this.viewerId,
    required this.userId,
    required this.tab,
    this.onFollowChanged,
  });

  final String token;
  final String viewerId;
  final String userId;
  final FollowListTab tab;
  final VoidCallback? onFollowChanged;

  @override
  State<_PeopleView> createState() => _PeopleViewState();
}

class _PeopleViewState extends State<_PeopleView> {
  final ScrollController _scroll = ScrollController();
  final List<FollowUser> _people = [];

  String? _cursor;
  String? _error;
  String? _nextPageError;
  bool _loading = true;
  bool _loadingMore = false;

  FollowsApi get _api => FollowsApi(apiClient(token: widget.token));

  @override
  void initState() {
    super.initState();
    usage.screen('follows');
    _scroll.addListener(_onScroll);
    _refresh();
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (!_scroll.hasClients || _cursor == null || _loadingMore) {
      return;
    }
    final left = _scroll.position.maxScrollExtent - _scroll.position.pixels;
    if (left < profileLoadAheadPixels) {
      _loadMore();
    }
  }

  Future<FollowList?> _page(String? cursor) =>
      widget.tab == FollowListTab.followers
      ? _api.getFollowers(widget.userId, limit: _pageSize, cursor: cursor)
      : _api.getFollowing(widget.userId, limit: _pageSize, cursor: cursor);

  Future<void> _refresh() async {
    setState(() {
      _error = null;
      _nextPageError = null;
      _loading = _people.isEmpty;
    });
    try {
      final page = await _page(null);
      debugPrint(
        '$logMarker follow_list=${widget.tab.name} '
        'people=${page?.items.length}',
      );
      if (!mounted) {
        return;
      }
      setState(() {
        _people
          ..clear()
          ..addAll(page?.items ?? const []);
        _cursor = page?.nextCursor;
        _loading = false;
      });
    } on Exception catch (error) {
      if (!mounted) {
        return;
      }
      setState(() {
        _loading = false;
        _error = errorMessage(error);
      });
    }
  }

  Future<void> _loadMore() async {
    final cursor = _cursor;
    if (cursor == null || _loadingMore) {
      return;
    }
    setState(() {
      _loadingMore = true;
      _nextPageError = null;
    });
    try {
      final page = await _page(cursor);
      if (!mounted) {
        return;
      }
      setState(() {
        _people.addAll(page?.items ?? const []);
        _cursor = page?.nextCursor;
        _loadingMore = false;
      });
    } on Exception catch (error) {
      if (!mounted) {
        return;
      }
      setState(() {
        _loadingMore = false;
        _nextPageError = errorMessage(error);
      });
    }
  }

  Future<void> _open(FollowUser person) => openUserProfile(
    context,
    token: widget.token,
    viewerId: widget.viewerId,
    userId: person.id,
    onFollowChanged: widget.onFollowChanged,
  );

  @override
  Widget build(BuildContext context) {
    final error = _error;

    if (_loading) {
      return const LoadingView(label: 'Открываю список…');
    }
    if (error != null && _people.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(AppGap.large),
        child: ErrorView(message: error, onRetry: _refresh),
      );
    }
    if (_people.isEmpty) {
      return RefreshIndicator(
        onRefresh: _refresh,
        child: ListView(
          children: [
            SizedBox(
              height: MediaQuery.sizeOf(context).height * 0.5,
              child: EmptyView(
                icon: Icons.people_outline,
                title: widget.tab == FollowListTab.followers
                    ? 'Подписчиков пока нет'
                    : 'Подписок пока нет',
              ),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _refresh,
      child: ListView.builder(
        controller: _scroll,
        itemCount: _people.length + 1,
        itemBuilder: (context, index) {
          if (index == _people.length) {
            return _footer();
          }
          final person = _people[index];
          final relation = person.relation;
          return PersonRow(
            key: ValueKey(person.id),
            nickname: person.nickname,
            name: person.name,
            avatarUrl: person.avatarUrl,
            subtitle: person.id == widget.viewerId ? 'Это вы' : null,
            onTap: () => _open(person),
            trailing: relation == null
                ? null
                : FollowButton(
                    token: widget.token,
                    userId: person.id,
                    relation: relation,
                    compact: true,
                    onChanged: (updated) {
                      setState(() => person.relation = updated);
                      widget.onFollowChanged?.call();
                    },
                  ),
          );
        },
      ),
    );
  }

  Widget _footer() {
    final error = _nextPageError;
    if (error != null) {
      return Padding(
        padding: const EdgeInsets.all(AppGap.large),
        child: ErrorView(message: error, onRetry: _loadMore),
      );
    }
    if (_loadingMore) {
      return const Padding(
        padding: EdgeInsets.all(AppGap.large),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    return const SizedBox(height: AppGap.large);
  }
}
