// Кнопка подписки: «Подписаться», «Вы подписаны», «Заявка отправлена»
// (specs/012-follows.md, требования 2–3 и 21).
//
// Одна и та же в профиле (во всю ширину) и в строке списка (узкая).
// Что на ней написано, решает отношение смотрящего, которое пришло от
// сервиса; после касания кнопка показывает то, что сервис ответил,
// а не то, что приложение ожидало.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';

/// Ширина узкой кнопки в строке списка.
const _compactWidth = 150.0;

class FollowButton extends StatefulWidget {
  const FollowButton({
    super.key,
    required this.token,
    required this.userId,
    required this.relation,
    required this.onChanged,
    this.compact = false,
  });

  final String token;

  /// На кого подписываемся.
  final String userId;

  /// Как смотрящий относится к нему сейчас.
  final Relation relation;

  /// Сервис ответил новым отношением.
  final ValueChanged<Relation> onChanged;

  /// Узкая кнопка для строки списка.
  final bool compact;

  @override
  State<FollowButton> createState() => _FollowButtonState();
}

class _FollowButtonState extends State<FollowButton> {
  bool _busy = false;

  FollowsApi get _api => FollowsApi(apiClient(token: widget.token));

  Future<void> _send(Future<Relation?> Function() request) async {
    setState(() => _busy = true);
    try {
      final relation = await request();
      debugPrint(
        '$logMarker follow user=${widget.userId} '
        'following=${relation?.following}',
      );
      if (relation != null) {
        widget.onChanged(relation);
      }
    } on Exception catch (error) {
      debugPrint('$logMarker follow=failed error=$error');
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(errorMessage(error))));
      }
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  Future<void> _follow() => _send(() => _api.followUser(widget.userId));

  Future<void> _unfollow() => _send(() => _api.unfollowUser(widget.userId));

  /// «Вы подписаны» не отписывает с одного касания: сначала выбор
  /// «Отписаться» (требование 21).
  Future<void> _offerUnfollow() async {
    final agreed = await showModalBottomSheet<bool>(
      context: context,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: ListTile(
          leading: const Icon(Icons.person_remove_outlined),
          title: const Text('Отписаться'),
          onTap: () => Navigator.of(context).pop(true),
        ),
      ),
    );
    if (agreed == true) {
      await _unfollow();
    }
  }

  @override
  Widget build(BuildContext context) {
    final compact = widget.compact;
    final Widget button = switch (widget.relation.following) {
      RelationFollowingEnum.yes => OutlinedButton.icon(
        onPressed: _busy ? null : _offerUnfollow,
        // Стрелка справа: касание не действие, а выбор.
        iconAlignment: IconAlignment.end,
        icon: const Icon(Icons.expand_more),
        label: const Text('Вы подписаны', maxLines: 1),
      ),
      // Повторное касание заявку отменяет (сценарий, шаг 8).
      RelationFollowingEnum.requested => OutlinedButton(
        onPressed: _busy ? null : _unfollow,
        child: Text(compact ? 'Запрошено' : 'Заявка отправлена', maxLines: 1),
      ),
      _ => FilledButton(
        onPressed: _busy ? null : _follow,
        child: const Text('Подписаться', maxLines: 1),
      ),
    };

    if (compact) {
      return SizedBox(
        width: _compactWidth,
        child: FittedBox(fit: BoxFit.scaleDown, child: button),
      );
    }
    return Padding(
      padding: const EdgeInsets.only(top: AppGap.medium),
      child: SizedBox(width: double.infinity, child: button),
    );
  }
}
