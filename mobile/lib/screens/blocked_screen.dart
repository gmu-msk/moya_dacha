// Мои блокировки: specs/022-edit-block-delete.md, требование 18.
//
// Кого человек заблокировал, последние сверху, и «Разблокировать» в
// каждой строке. Список приходит целиком: их единицы.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/empty_view.dart';
import '../widgets/error_view.dart';
import '../widgets/loading_view.dart';
import '../widgets/person_row.dart';

class BlockedScreen extends StatefulWidget {
  const BlockedScreen({super.key, required this.token});

  final String token;

  @override
  State<BlockedScreen> createState() => _BlockedScreenState();
}

class _BlockedScreenState extends State<BlockedScreen> {
  List<Author>? _people;
  String? _error;

  /// Кого сейчас разблокируют: его кнопка ждёт ответа.
  final Set<String> _busy = {};

  FollowsApi get _api => FollowsApi(apiClient(token: widget.token));

  @override
  void initState() {
    super.initState();
    usage.screen('blocked');
    _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final list = await _api.getBlocked();
      debugPrint('$logMarker blocked=loaded count=${list?.items.length}');
      if (!mounted) {
        return;
      }
      setState(() => _people = list?.items ?? const []);
    } on Exception catch (error) {
      debugPrint('$logMarker blocked=failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() => _error = errorMessage(error));
    }
  }

  Future<void> _unblock(Author person) async {
    setState(() => _busy.add(person.id));
    try {
      await _api.unblockUser(person.id);
      debugPrint('$logMarker user=unblocked id=${person.id}');
      if (!mounted) {
        return;
      }
      setState(
        () => _people = [
          for (final item in _people ?? const <Author>[])
            if (item.id != person.id) item,
        ],
      );
    } on Exception catch (error) {
      debugPrint('$logMarker user=unblock_failed error=$error');
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(errorMessage(error))));
    } finally {
      if (mounted) {
        setState(() => _busy.remove(person.id));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final people = _people;
    final error = _error;

    final Widget body;
    if (error != null) {
      body = ErrorView(message: error, onRetry: _load);
    } else if (people == null) {
      body = const LoadingView(label: 'Открываю список…');
    } else if (people.isEmpty) {
      body = const EmptyView(
        icon: Icons.block_outlined,
        title: 'Вы никого не блокировали',
      );
    } else {
      body = ListView(
        children: [
          for (final person in people)
            PersonRow(
              nickname: person.nickname,
              name: person.name,
              avatarUrl: person.avatarUrl,
              trailing: OutlinedButton(
                onPressed: _busy.contains(person.id)
                    ? null
                    : () => _unblock(person),
                child: const Text('Разблокировать'),
              ),
            ),
        ],
      );
    }

    return AppScreen(
      title: 'Заблокированные',
      showServerStatus: false,
      padded: false,
      child: body,
    );
  }
}
