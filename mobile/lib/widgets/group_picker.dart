// «Выложить в группе» в новом посте: specs/030-group-posts.md,
// требование 16.
//
// Галочка, под ней — свои группы с галочками и напоминание, что «Кто
// увидит» главнее. Групп нет или список не пришёл — ничего не видно:
// публикации это не мешает. Список грузит экран нового поста: он нужен
// и «Кто увидит» (specs/031-group-visibility.md, требование 14).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../theme.dart';

class GroupPicker extends StatefulWidget {
  const GroupPicker({
    super.key,
    required this.groups,
    required this.onChanged,
    this.lockedId,
    this.enabled = true,
  });

  /// Свои группы (029, `scope=mine`).
  final List<Group> groups;

  /// Группа из «Кто увидит»: пост сам выкладывается в неё, поэтому она
  /// отмечена вместе с галочкой и снять её нельзя
  /// (specs/031-group-visibility.md, требование 16).
  final String? lockedId;
  final bool enabled;

  /// Отмеченные группы; галочка «Выложить в группе» снята — пусто.
  final ValueChanged<List<String>> onChanged;

  @override
  State<GroupPicker> createState() => _GroupPickerState();
}

class _GroupPickerState extends State<GroupPicker> {
  bool _on = false;
  final Set<String> _picked = {};

  bool get _effectiveOn => _on || widget.lockedId != null;

  bool _checked(String id) => _picked.contains(id) || id == widget.lockedId;

  @override
  void didUpdateWidget(GroupPicker old) {
    super.didUpdateWidget(old);
    if (old.lockedId != widget.lockedId) {
      _report();
    }
  }

  void _report() => widget.onChanged(
    _effectiveOn
        ? [
            for (final g in widget.groups)
              if (_checked(g.id)) g.id,
          ]
        : [],
  );

  @override
  Widget build(BuildContext context) {
    final groups = widget.groups;
    if (groups.isEmpty) {
      return const SizedBox.shrink();
    }
    final theme = Theme.of(context);
    final enabled = widget.enabled;
    final locked = widget.lockedId;

    return AnimatedSize(
      duration: AppMotion.standard,
      curve: AppMotion.ease,
      alignment: Alignment.topCenter,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          CheckboxListTile(
            value: _effectiveOn,
            contentPadding: EdgeInsets.zero,
            controlAffinity: ListTileControlAffinity.leading,
            title: const Text('Выложить в группе'),
            onChanged: enabled && locked == null
                ? (value) {
                    setState(() => _on = value ?? false);
                    _report();
                  }
                : null,
          ),
          if (_effectiveOn) ...[
            for (final group in groups)
              CheckboxListTile(
                value: _checked(group.id),
                dense: true,
                contentPadding: const EdgeInsets.only(left: AppGap.large),
                controlAffinity: ListTileControlAffinity.leading,
                title: Text(group.name),
                onChanged: enabled && group.id != locked
                    ? (value) {
                        setState(() {
                          if (value ?? false) {
                            _picked.add(group.id);
                          } else {
                            _picked.remove(group.id);
                          }
                        });
                        _report();
                      }
                    : null,
              ),
            Padding(
              padding: const EdgeInsets.only(
                left: AppGap.large,
                top: AppGap.tiny,
              ),
              child: Text(
                'Пост увидят участники, которым он виден по «Кто увидит»',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }
}
