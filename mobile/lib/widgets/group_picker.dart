// «Выложить в группе» в новом посте: specs/030-group-posts.md,
// требование 16.
//
// Галочка, под ней — свои группы с галочками и напоминание, что «Кто
// увидит» главнее. Групп нет или список не пришёл — ничего не видно:
// публикации это не мешает.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';

class GroupPicker extends StatefulWidget {
  const GroupPicker({
    super.key,
    required this.token,
    required this.onChanged,
    this.enabled = true,
  });

  final String token;
  final bool enabled;

  /// Отмеченные группы; галочка «Выложить в группе» снята — пусто.
  final ValueChanged<List<String>> onChanged;

  @override
  State<GroupPicker> createState() => _GroupPickerState();
}

class _GroupPickerState extends State<GroupPicker> {
  List<Group> _groups = const [];
  bool _on = false;
  final Set<String> _picked = {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final list = await GroupsApi(apiClient(token: widget.token))
          .getGroups(scope: 'mine');
      debugPrint('$logMarker post=groups count=${list?.items.length}');
      if (mounted) {
        setState(() => _groups = list?.items ?? const []);
      }
    } on Exception catch (error) {
      debugPrint('$logMarker post=groups_failed error=$error');
    }
  }

  void _report() => widget.onChanged(
    _on
        ? [
            for (final g in _groups)
              if (_picked.contains(g.id)) g.id,
          ]
        : [],
  );

  @override
  Widget build(BuildContext context) {
    if (_groups.isEmpty) {
      return const SizedBox.shrink();
    }
    final theme = Theme.of(context);
    final enabled = widget.enabled;

    return AnimatedSize(
      duration: AppMotion.standard,
      curve: AppMotion.ease,
      alignment: Alignment.topCenter,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          CheckboxListTile(
            value: _on,
            contentPadding: EdgeInsets.zero,
            controlAffinity: ListTileControlAffinity.leading,
            title: const Text('Выложить в группе'),
            onChanged: enabled
                ? (value) {
                    setState(() => _on = value ?? false);
                    _report();
                  }
                : null,
          ),
          if (_on) ...[
            for (final group in _groups)
              CheckboxListTile(
                value: _picked.contains(group.id),
                dense: true,
                contentPadding: const EdgeInsets.only(left: AppGap.large),
                controlAffinity: ListTileControlAffinity.leading,
                title: Text(group.name),
                onChanged: enabled
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
