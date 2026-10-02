// Строка групп поста: specs/030-group-posts.md, требование 17.
//
// «в группе снт Ромашка» под автором; название — цветом ссылки, касание
// открывает группу. Групп несколько — «в группах X, Y».
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../theme.dart';

class PostGroupsLine extends StatelessWidget {
  const PostGroupsLine({super.key, required this.groups, this.onOpen});

  final List<GroupBrief> groups;
  final void Function(GroupBrief group)? onOpen;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = theme.colorScheme.onSurfaceVariant;
    final quiet = theme.textTheme.bodyMedium?.copyWith(color: color);
    final link = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.primary,
      fontWeight: FontWeight.w600,
    );
    final open = onOpen;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(right: AppGap.tiny, top: AppGap.tiny),
          child: Icon(Icons.groups_outlined, size: 18, color: color),
        ),
        Expanded(
          child: Wrap(
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Padding(
                padding: const EdgeInsets.symmetric(vertical: AppGap.tiny),
                child: Text(
                  groups.length == 1 ? 'в группе ' : 'в группах ',
                  style: quiet,
                ),
              ),
              for (final (index, group) in groups.indexed)
                Semantics(
                  button: open != null,
                  label: 'Группа ${group.name}',
                  excludeSemantics: true,
                  child: InkWell(
                    onTap: open == null ? null : () => open(group),
                    borderRadius: BorderRadius.circular(AppShape.small),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        vertical: AppGap.tiny,
                      ),
                      child: Text.rich(
                        TextSpan(
                          children: [
                            TextSpan(text: group.name, style: link),
                            if (index < groups.length - 1)
                              TextSpan(text: ', ', style: quiet),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }
}
