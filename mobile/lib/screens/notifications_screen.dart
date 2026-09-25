// Раздел «Уведомления»: specs/014-notifications.md.
//
// Сверху — заявки на подписку к своему закрытому профилю, ниже — события,
// новые сверху, разделённые на «Новое» и «Раньше». Открытие раздела
// отмечает всё прочитанным (требование 7): что было новым в этот момент,
// остаётся под «Новым», пока раздел не откроют снова.
import 'package:flutter/material.dart' hide Notification;
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/author_line.dart';
import '../widgets/bottom_bar.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/follow_button.dart';
import '../widgets/loading_view.dart';
import '../widgets/user_avatar.dart';

/// Сколько строк приходит за раз.
const _pageSize = 20;

/// За сколько пикселей до конца списка просить следующую страницу.
const _loadAheadPixels = 600.0;

/// Миниатюра поста в строке: квадрат 48 со скруглением как у фото.
const _thumbnailSize = 48.0;
const _thumbnailRadius = AppShape.small;

/// Отступ кнопок заявки: под текстом, а не под аватаром.
const _requestButtonsIndent =
    AppGap.medium + AvatarRadius.inPost * 2 + AppGap.small + AppGap.tiny;

class NotificationsScreen extends StatefulWidget {
  const NotificationsScreen({
    super.key,
    required this.token,
    required this.viewerId,
    required this.onOpenPost,
    required this.onOpenProfile,
    this.onSeen,
  });

  final String token;
  final String viewerId;

  /// Касание строки с постом (требование 4).
  final void Function(String postId) onOpenPost;

  /// Касание строки с человеком.
  final void Function(String userId) onOpenProfile;

  /// Раздел открыт и всё отмечено прочитанным, или заявок стало меньше:
  /// точку на колокольчике пора перепроверить.
  final VoidCallback? onSeen;

  @override
  State<NotificationsScreen> createState() => NotificationsScreenState();
}

class NotificationsScreenState extends State<NotificationsScreen> {
  final ScrollController _scroll = ScrollController();
  final List<Author> _requests = [];
  final List<Notification> _items = [];
  final Set<String> _busy = {};

  String? _cursor;
  String? _error;
  bool _loading = true;
  bool _loadingMore = false;

  NotificationsApi get _api => NotificationsApi(apiClient(token: widget.token));
  FollowsApi get _follows => FollowsApi(apiClient(token: widget.token));

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_onScroll);
    refresh();
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  /// К самому верху: повторное касание колокольчика
  /// (specs/011-bottom-bar.md, требование 4).
  Future<void> scrollToTop() => scrollBackToTop(_scroll);

  void _onScroll() {
    if (!_scroll.hasClients || _cursor == null || _loadingMore) {
      return;
    }
    final left = _scroll.position.maxScrollExtent - _scroll.position.pixels;
    if (left < _loadAheadPixels) {
      _loadMore();
    }
  }

  /// Заявки и первая страница событий заново, после чего всё прочитано.
  /// Так раздел ведёт себя при каждом открытии и когда его тянут вниз.
  Future<void> refresh() async {
    setState(() {
      _error = null;
      _loading = _items.isEmpty && _requests.isEmpty;
    });
    try {
      // Одно за другим: два коротких запроса, а ошибка любого из них —
      // одна ошибка раздела.
      final requests = await _follows.getFollowRequests(limit: 50);
      final page = await _api.getNotifications(limit: _pageSize);
      debugPrint(
        '$logMarker screen=notifications requests=${requests?.items.length} '
        'items=${page?.items.length} '
        'unread=${page?.items.where((item) => item.unread).length}',
      );
      if (!mounted) {
        return;
      }
      setState(() {
        _requests
          ..clear()
          ..addAll(requests?.items ?? const []);
        _items
          ..clear()
          ..addAll(page?.items ?? const []);
        _cursor = page?.nextCursor;
        _loading = false;
      });
      await _api.markNotificationsSeen();
      widget.onSeen?.call();
    } on Exception catch (error) {
      _failed(error);
    }
  }

  void _failed(Object error) {
    debugPrint('$logMarker screen=notifications error=$error');
    if (!mounted) {
      return;
    }
    setState(() {
      _loading = false;
      _error = errorMessage(error);
    });
  }

  Future<void> _loadMore() async {
    final cursor = _cursor;
    if (cursor == null || _loadingMore) {
      return;
    }
    setState(() => _loadingMore = true);
    try {
      final page = await _api.getNotifications(
        limit: _pageSize,
        cursor: cursor,
      );
      if (!mounted) {
        return;
      }
      setState(() {
        _items.addAll(page?.items ?? const []);
        _cursor = page?.nextCursor;
      });
    } on Exception catch (error) {
      _say(errorMessage(error));
    } finally {
      if (mounted) {
        setState(() => _loadingMore = false);
      }
    }
  }

  /// Принять или отклонить заявку. Строка уходит, когда сервис ответил;
  /// заявку, которую успели отменить, тоже убираем — её больше нет.
  Future<void> _answer(Author person, {required bool accept}) async {
    setState(() => _busy.add(person.id));
    try {
      if (accept) {
        await _follows.acceptFollowRequest(person.id);
      } else {
        await _follows.declineFollowRequest(person.id);
      }
      debugPrint(
        '$logMarker follow_request=${accept ? 'accepted' : 'declined'} '
        'user=${person.id}',
      );
      _gone(person);
    } on Exception catch (error) {
      if (serviceErrorCode(error) == 'request_not_found') {
        _gone(person);
      } else {
        _say(errorMessage(error));
      }
    } finally {
      if (mounted) {
        setState(() => _busy.remove(person.id));
      }
    }
  }

  void _gone(Author person) {
    if (mounted) {
      setState(() => _requests.removeWhere((item) => item.id == person.id));
      widget.onSeen?.call();
    }
  }

  /// Подписались в ответ из строки «подписался на вас»: у всех его строк
  /// теперь другая кнопка.
  void _relationChanged(FollowUser actor, Relation relation) {
    setState(() {
      for (final item in _items) {
        if (item.actor.id == actor.id) {
          item.actor.relation = relation;
        }
      }
    });
  }

  void _say(String message) {
    if (mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(message)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final error = _error;

    final Widget body;
    if (_loading) {
      body = const LoadingView(label: 'Открываю уведомления…');
    } else if (error != null && _items.isEmpty && _requests.isEmpty) {
      body = Padding(
        padding: const EdgeInsets.all(AppGap.large),
        child: ErrorView(message: error, onRetry: refresh),
      );
    } else if (_items.isEmpty && _requests.isEmpty) {
      body = RefreshIndicator(
        onRefresh: refresh,
        child: LayoutBuilder(
          builder: (context, constraints) => SingleChildScrollView(
            controller: _scroll,
            physics: const AlwaysScrollableScrollPhysics(),
            child: SizedBox(
              height: constraints.maxHeight,
              child: const EmptyView(
                icon: Icons.notifications_none,
                title: 'Здесь появятся отметки, комментарии и новые подписчики',
              ),
            ),
          ),
        ),
      );
    } else {
      final fresh = _items.where((item) => item.unread).toList();
      final earlier = _items.where((item) => !item.unread).toList();
      body = RefreshIndicator(
        onRefresh: refresh,
        child: ListView(
          controller: _scroll,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.only(bottom: AppGap.medium),
          children: [
            if (_requests.isNotEmpty) ...[
              const _SectionTitle('Заявки на подписку'),
              for (final person in _requests) _requestRow(person),
              if (_items.isNotEmpty) const _Divider(),
            ],
            if (fresh.isNotEmpty) ...[
              const _SectionTitle('Новое'),
              for (final item in fresh) _eventRow(item),
            ],
            if (earlier.isNotEmpty) ...[
              const _SectionTitle('Раньше'),
              for (final item in earlier) _eventRow(item),
            ],
            if (_loadingMore)
              const Padding(
                padding: EdgeInsets.all(AppGap.medium),
                child: Center(child: CircularProgressIndicator()),
              ),
          ],
        ),
      );
    }

    return AppScreen(title: 'Уведомления', padded: false, child: body);
  }

  Widget _requestRow(Author person) {
    final busy = _busy.contains(person.id);

    return Column(
      key: ValueKey('request-${person.id}'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _Row(
          nickname: person.nickname,
          avatarUrl: person.avatarUrl,
          text: 'хочет подписаться на вас',
          onTap: () => widget.onOpenProfile(person.id),
        ),
        // «Принять» — маковая, «Отклонить» — с контуром, обе под текстом
        // заявки (требование 10).
        Padding(
          padding: const EdgeInsets.fromLTRB(
            _requestButtonsIndent,
            0,
            AppGap.medium,
            AppGap.small,
          ),
          child: Wrap(
            spacing: AppGap.small,
            runSpacing: AppGap.small,
            children: [
              FilledButton(
                onPressed: busy ? null : () => _answer(person, accept: true),
                child: const Text('Принять'),
              ),
              OutlinedButton(
                onPressed: busy ? null : () => _answer(person, accept: false),
                child: const Text('Отклонить'),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _eventRow(Notification item) {
    final actor = item.actor;
    final post = item.post;
    final relation = actor.relation;

    final Widget? trailing;
    if (post != null) {
      trailing = ClipRRect(
        borderRadius: BorderRadius.circular(_thumbnailRadius),
        child: SizedBox.square(
          dimension: _thumbnailSize,
          child: Image.network(
            mediaUrl(post.thumbnail.url),
            fit: BoxFit.cover,
            cacheWidth: 144,
            errorBuilder: (context, _, _) => ColoredBox(
              color: Theme.of(context).colorScheme.surfaceContainerHighest,
            ),
          ),
        ),
      );
    } else if (item.kind == NotificationKindEnum.follow && relation != null) {
      trailing = FollowButton(
        token: widget.token,
        userId: actor.id,
        relation: relation,
        compact: true,
        onChanged: (updated) => _relationChanged(actor, updated),
      );
    } else {
      trailing = null;
    }

    return _Row(
      key: ValueKey(item.id),
      nickname: actor.nickname,
      avatarUrl: actor.avatarUrl,
      text: notificationText(item),
      when: item.createdAt,
      trailing: trailing,
      onTap: post != null
          ? () => widget.onOpenPost(post.id)
          : () => widget.onOpenProfile(actor.id),
    );
  }
}

/// Что случилось — текст строки после ника (требование 1).
String notificationText(Notification item) {
  final others = item.others ?? 0;
  return switch (item.kind) {
    NotificationKindEnum.follow => 'подписался на вас',
    NotificationKindEnum.followAccepted => 'принял вашу заявку',
    NotificationKindEnum.like when others > 0 =>
      'и ещё $others отметили ваш пост',
    NotificationKindEnum.like => 'отметил ваш пост',
    NotificationKindEnum.comment =>
      'ответил на ваш пост: «${item.comment ?? ''}»',
  };
}

/// Строка раздела: аватар, ник васильком, что случилось, время
/// приглушённым после текста и справа миниатюра или кнопка
/// (требования 3 и 10).
class _Row extends StatelessWidget {
  const _Row({
    super.key,
    required this.nickname,
    required this.avatarUrl,
    required this.text,
    required this.onTap,
    this.when,
    this.trailing,
  });

  final String nickname;
  final String? avatarUrl;
  final String text;
  final DateTime? when;
  final Widget? trailing;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    final when = this.when;
    final trailing = this.trailing;

    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppGap.medium,
          vertical: AppGap.small,
        ),
        child: Row(
          children: [
            Avatar(
              name: nickname,
              link: avatarUrl,
              radius: AvatarRadius.inPost,
            ),
            const SizedBox(width: AppGap.small + AppGap.tiny),
            Expanded(
              child: Text.rich(
                TextSpan(
                  style: theme.textTheme.bodyLarge,
                  children: [
                    TextSpan(
                      text: nickname,
                      style: theme.textTheme.titleMedium?.copyWith(
                        color: colors.secondary,
                      ),
                    ),
                    TextSpan(text: ' $text'),
                    if (when != null)
                      TextSpan(
                        text: ' ${whenPosted(when)}',
                        style: TextStyle(color: colors.onSurfaceVariant),
                      ),
                  ],
                ),
              ),
            ),
            if (trailing != null) ...[
              const SizedBox(width: AppGap.small + AppGap.tiny),
              trailing,
            ],
          ],
        ),
      ),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle(this.text);

  final String text;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppGap.medium,
        AppGap.small + AppGap.tiny,
        AppGap.medium,
        AppGap.tiny,
      ),
      child: Semantics(
        header: true,
        child: Text(
          text,
          style: theme.textTheme.titleMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ),
    );
  }
}

class _Divider extends StatelessWidget {
  const _Divider();

  @override
  Widget build(BuildContext context) => const Padding(
    padding: EdgeInsets.symmetric(
      horizontal: AppGap.medium,
      vertical: AppGap.small,
    ),
    child: Divider(height: AppShape.hairline, thickness: AppShape.hairline),
  );
}
