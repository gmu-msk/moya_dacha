// Заявки на подписку к своему закрытому профилю: specs/012-follows.md,
// требование 10.
//
// Постоянное место заявкам — раздел «Уведомления»
// (specs/014-notifications.md). Пока его нет, они открываются из своего
// профиля: иначе закрытому профилю некуда смотреть, кто просится.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/loading_view.dart';
import '../widgets/person_row.dart';
import 'user_screen.dart';

class FollowRequestsScreen extends StatefulWidget {
  const FollowRequestsScreen({
    super.key,
    required this.token,
    required this.viewerId,
  });

  final String token;
  final String viewerId;

  @override
  State<FollowRequestsScreen> createState() => _FollowRequestsScreenState();
}

class _FollowRequestsScreenState extends State<FollowRequestsScreen> {
  final List<Author> _people = [];
  final Set<String> _busy = {};

  String? _cursor;
  String? _error;
  bool _loading = true;

  FollowsApi get _api => FollowsApi(apiClient(token: widget.token));

  @override
  void initState() {
    super.initState();
    _refresh();
  }

  Future<void> _refresh() async {
    setState(() {
      _error = null;
      _loading = _people.isEmpty;
    });
    try {
      final page = await _api.getFollowRequests(limit: 50);
      debugPrint('$logMarker follow_requests=${page?.items.length}');
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

  Future<void> _more() async {
    final cursor = _cursor;
    if (cursor == null) {
      return;
    }
    try {
      final page = await _api.getFollowRequests(limit: 50, cursor: cursor);
      if (!mounted) {
        return;
      }
      setState(() {
        _people.addAll(page?.items ?? const []);
        _cursor = page?.nextCursor;
      });
    } on Exception catch (error) {
      _say(errorMessage(error));
    }
  }

  /// Принять или отклонить. Строка уходит из списка, когда сервис
  /// ответил; заявку, которую успели отменить, тоже убираем — её больше
  /// нет.
  Future<void> _answer(Author person, {required bool accept}) async {
    setState(() => _busy.add(person.id));
    try {
      if (accept) {
        await _api.acceptFollowRequest(person.id);
      } else {
        await _api.declineFollowRequest(person.id);
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
      setState(() => _people.removeWhere((item) => item.id == person.id));
    }
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
      body = const LoadingView(label: 'Открываю заявки…');
    } else if (error != null && _people.isEmpty) {
      body = Padding(
        padding: const EdgeInsets.all(AppGap.large),
        child: ErrorView(message: error, onRetry: _refresh),
      );
    } else if (_people.isEmpty) {
      body = const EmptyView(
        icon: Icons.person_add_alt_outlined,
        title: 'Заявок нет',
      );
    } else {
      body = RefreshIndicator(
        onRefresh: _refresh,
        child: ListView(
          children: [
            for (final person in _people) _row(person),
            if (_cursor != null)
              Padding(
                padding: const EdgeInsets.all(AppGap.medium),
                child: OutlinedButton(
                  onPressed: _more,
                  child: const Text('Показать ещё'),
                ),
              ),
          ],
        ),
      );
    }

    return AppScreen(
      title: 'Заявки на подписку',
      showServerStatus: false,
      padded: false,
      child: body,
    );
  }

  Widget _row(Author person) {
    final busy = _busy.contains(person.id);

    return Column(
      key: ValueKey(person.id),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        PersonRow(
          nickname: person.nickname,
          name: person.name,
          avatarUrl: person.avatarUrl,
          onTap: () => openUserProfile(
            context,
            token: widget.token,
            viewerId: widget.viewerId,
            userId: person.id,
          ),
        ),
        // «Принять» — маковая, «Отклонить» — с контуром, обе под строкой
        // (specs/014-notifications.md, раздел «Вид»).
        Padding(
          padding: const EdgeInsets.fromLTRB(
            AppGap.medium,
            0,
            AppGap.medium,
            AppGap.medium,
          ),
          child: Row(
            children: [
              Expanded(
                child: FilledButton(
                  onPressed: busy ? null : () => _answer(person, accept: true),
                  child: const Text('Принять'),
                ),
              ),
              const SizedBox(width: AppGap.small),
              Expanded(
                child: OutlinedButton(
                  onPressed: busy ? null : () => _answer(person, accept: false),
                  child: const Text('Отклонить'),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
