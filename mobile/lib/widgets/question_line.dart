// Плашка вопроса и статус: specs/033-question-posts.md, требования 14–15.
//
// «Вопрос» и рядом «Решён» с галочкой или «Не решён» с часами — статус
// различается словом и значком, не только цветом (000-ui, правило 7).
// Сервер с main полей вопроса не знает — тогда пост обычный, плашки нет
// (требование 20).
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../theme.dart';

/// Пост — вопрос. Старый сервер поля не отдаёт: тогда не вопрос.
bool isQuestion(Post post) => post.question ?? false;

/// Плашка «Вопрос» и статус. У обычного поста — пусто.
class QuestionLine extends StatelessWidget {
  const QuestionLine({super.key, required this.post});

  final Post post;

  @override
  Widget build(BuildContext context) {
    if (!isQuestion(post)) {
      return const SizedBox.shrink();
    }
    final scheme = Theme.of(context).colorScheme;
    final solved = post.solved ?? false;

    return Wrap(
      spacing: AppGap.small,
      runSpacing: AppGap.tiny,
      children: [
        _Badge(
          icon: Icons.help_outline,
          label: 'Вопрос',
          background: scheme.secondaryContainer,
          foreground: scheme.onSecondaryContainer,
        ),
        if (solved)
          _Badge(
            icon: Icons.check_circle_outline,
            label: 'Решён',
            background: scheme.primaryContainer,
            foreground: scheme.onPrimaryContainer,
          )
        else
          _Badge(
            icon: Icons.schedule,
            label: 'Не решён',
            background: scheme.surfaceContainerHighest,
            foreground: scheme.onSurfaceVariant,
          ),
      ],
    );
  }
}

/// Метка «Решение» над комментарием-решением (требование 17).
class AnswerBadge extends StatelessWidget {
  const AnswerBadge({super.key});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return _Badge(
      icon: Icons.check_circle,
      label: 'Решение',
      background: scheme.primary,
      foreground: scheme.onPrimary,
    );
  }
}

class _Badge extends StatelessWidget {
  const _Badge({
    required this.icon,
    required this.label,
    required this.background,
    required this.foreground,
  });

  final IconData icon;
  final String label;
  final Color background;
  final Color foreground;

  @override
  Widget build(BuildContext context) {
    final style = Theme.of(context).textTheme.labelLarge
        ?.copyWith(color: foreground);
    return Semantics(
      label: label,
      excludeSemantics: true,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: background,
          borderRadius: BorderRadius.circular(AppShape.pill),
        ),
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppGap.small,
            vertical: AppGap.tiny,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, size: 18, color: foreground),
              const SizedBox(width: AppGap.tiny),
              Flexible(child: Text(label, style: style)),
            ],
          ),
        ),
      ),
    );
  }
}
