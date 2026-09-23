// Нижняя панель главного экрана: specs/011-bottom-bar.md.
//
// Три кнопки — «Лента», «Новый пост», «Профиль». Лента и профиль —
// разделы, новый пост — действие. Значки нарисованы кодом под «Ситец»,
// а не взяты из набора Material (требование 10): лента — штакетник из
// логотипа, новый пост — плюс на маке, профиль — свой аватар.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../theme.dart';
import 'app_logo.dart';
import 'user_avatar.dart';

/// Разделы главного экрана.
enum HomeTab { feed, profile }

class AppBottomBar extends StatelessWidget {
  const AppBottomBar({
    super.key,
    required this.tab,
    required this.user,
    required this.onSelect,
    required this.onNewPost,
  });

  /// Открытый раздел.
  final HomeTab tab;

  /// Кто вошёл: его аватар — значок профиля.
  final CurrentUser user;

  /// Касание раздела, в том числе уже открытого: тогда раздел
  /// возвращается к самому верху (требование 4).
  final void Function(HomeTab tab) onSelect;

  /// «Новый пост» — не раздел, а действие (требование 5).
  final VoidCallback onNewPost;

  /// Порядок кнопок в панели.
  static const _feed = 0;
  static const _newPost = 1;
  static const _profile = 2;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final labels = Theme.of(context).textTheme.labelMedium;

    return DecoratedBox(
      // Кант сверху — как у карточки поста; тени нет (требование 12).
      decoration: BoxDecoration(
        border: Border(
          top: BorderSide(
            color: colors.outlineVariant,
            width: AppShape.hairline,
          ),
        ),
      ),
      child: NavigationBarTheme(
        data: NavigationBarThemeData(
          backgroundColor: colors.surface,
          surfaceTintColor: Colors.transparent,
          elevation: 0,
          // Выбранное видно по самому значку, «таблетка» под ним лишняя.
          indicatorColor: Colors.transparent,
          labelTextStyle: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.selected)
                ? labels?.copyWith(
                    color: colors.onSurface,
                    fontWeight: FontWeight.w700,
                  )
                : labels?.copyWith(color: colors.onSurfaceVariant),
          ),
        ),
        child: NavigationBar(
          selectedIndex: tab == HomeTab.feed ? _feed : _profile,
          labelBehavior: NavigationDestinationLabelBehavior.alwaysShow,
          onDestinationSelected: (index) => switch (index) {
            _feed => onSelect(HomeTab.feed),
            _newPost => onNewPost(),
            _ => onSelect(HomeTab.profile),
          },
          destinations: [
            const NavigationDestination(
              icon: FenceIcon(selected: false),
              selectedIcon: FenceIcon(selected: true),
              label: 'Лента',
            ),
            const NavigationDestination(
              icon: NewPostIcon(),
              label: 'Новый пост',
            ),
            NavigationDestination(
              icon: ProfileIcon(user: user, selected: false),
              selectedIcon: ProfileIcon(user: user, selected: true),
              label: 'Профиль',
            ),
          ],
        ),
      ),
    );
  }
}

/// Размер значков панели: чуть крупнее материаловых 24, рисунки у нас
/// мельче по деталям. Все три значка одного размера, иначе подписи под
/// ними встают вразнобой.
const _iconSize = 30.0;

/// Лента — штакетник из логотипа. Раздел закрыт — контур, открыт —
/// штакетник залит васильком и из-за него выглядывает мак.
class FenceIcon extends StatelessWidget {
  const FenceIcon({super.key, required this.selected});

  final bool selected;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;

    return CustomPaint(
      size: const Size.square(_iconSize),
      painter: FenceMarkPainter(
        fence: selected ? colors.secondary : colors.onSurfaceVariant,
        poppy: colors.primary,
        poppyHeart: colors.onSurface,
        stem: AppBrand.stem(theme.brightness),
        outline: !selected,
      ),
    );
  }
}

/// Новый пост — белый плюс на маковом квадрате со скруглением темы.
class NewPostIcon extends StatelessWidget {
  const NewPostIcon({super.key});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;

    return DecoratedBox(
      decoration: BoxDecoration(
        color: colors.primary,
        // Скругление кнопки темы, уменьшенное вместе с самой кнопкой:
        // 16 на 56 — то же, что здесь на размере значка.
        borderRadius: BorderRadius.circular(_iconSize * 16 / 56),
      ),
      child: SizedBox.square(
        dimension: _iconSize,
        child: Icon(Icons.add, color: colors.onPrimary, size: _iconSize - 6),
      ),
    );
  }
}

/// Профиль — свой аватар. Открыт — вокруг него васильковое кольцо; место
/// под кольцо есть всегда, чтобы значок не прыгал: аватар с кольцом и
/// зазором ровно в размер значка.
class ProfileIcon extends StatelessWidget {
  const ProfileIcon({super.key, required this.user, required this.selected});

  final CurrentUser user;
  final bool selected;

  static const _ring = 2.0;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;

    return Container(
      width: _iconSize,
      height: _iconSize,
      padding: const EdgeInsets.all(_ring),
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(
          color: selected ? colors.secondary : Colors.transparent,
          width: _ring,
        ),
      ),
      child: UserAvatar(user: user, radius: AvatarRadius.inBottomBar),
    );
  }
}

/// Время, за которое раздел доезжает до верха.
const _toTopDuration = Duration(milliseconds: 350);

/// Вернуть раздел к самому верху — повторное касание в панели
/// (требование 4). Если долистали далеко, сначала прыжок поближе:
/// иначе прокрутка через сотню постов тянется и дёргается.
Future<void> scrollBackToTop(ScrollController scroll) async {
  if (!scroll.hasClients) {
    return;
  }
  final position = scroll.position;
  final near = position.viewportDimension * 2;
  if (position.pixels > near) {
    scroll.jumpTo(near);
  }
  await scroll.animateTo(0, duration: _toTopDuration, curve: Curves.easeOut);
}
