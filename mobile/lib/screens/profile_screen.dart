// Правка своего профиля: никнейм, полное имя, «о себе» и аватар
// (specs/002-profile.md, specs/010-nicknames.md).
//
// Всё, что здесь видно, принадлежит владельцу токена, и номер телефона
// показывается только здесь. Открывается кнопкой «Изменить профиль» из
// своего профиля (specs/009-user-profile.md), который выглядит так же,
// как чужой.
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' show MultipartFile;
import 'package:image_picker/image_picker.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';
import '../widgets/user_avatar.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({
    super.key,
    required this.token,
    required this.user,
    required this.onSignedOut,
  });

  final String token;
  final CurrentUser user;

  /// Выход: токен забывает и приложение, и сервис. Выход живёт здесь,
  /// а не в ленте: там место постам (specs/004-feed.md).
  final Future<void> Function() onSignedOut;

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  late CurrentUser _user = widget.user;
  late final TextEditingController _nickname = TextEditingController(
    text: _user.nickname,
  );
  late final TextEditingController _name = TextEditingController(
    text: _user.name,
  );
  late final TextEditingController _about = TextEditingController(
    text: _user.about,
  );

  /// Ошибка никнейма — под его полем, остальные — под кнопкой.
  String? _nicknameError;
  String? _error;
  String? _saved;
  bool _busy = false;

  ProfileApi get _api => ProfileApi(apiClient(token: widget.token));

  @override
  void dispose() {
    _nickname.dispose();
    _name.dispose();
    _about.dispose();
    super.dispose();
  }

  /// Каждое изменение профиля возвращает профиль целиком, поэтому экран
  /// не собирает его по кусочкам, а показывает то, что ответил сервис.
  Future<void> _change(
    Future<CurrentUser?> Function() request,
    String done,
  ) async {
    setState(() {
      _busy = true;
      _nicknameError = null;
      _error = null;
      _saved = null;
    });

    try {
      final user = await request();
      if (user == null) {
        throw ApiException(200, 'Сервис не вернул профиль');
      }
      debugPrint('$logMarker profile=changed avatar=${user.avatarUrl}');
      if (!mounted) {
        return;
      }
      setState(() {
        _user = user;
        _nickname.text = user.nickname;
        _name.text = user.name;
        _about.text = user.about;
        _busy = false;
        _saved = done;
      });
    } on Exception catch (error) {
      debugPrint('$logMarker profile=change_failed error=$error');
      if (!mounted) {
        return;
      }
      final code = serviceErrorCode(error);
      setState(() {
        _busy = false;
        if (code == 'invalid_nickname' || code == 'nickname_taken') {
          _nicknameError = errorMessage(error);
        } else {
          _error = errorMessage(error);
        }
      });
    }
  }

  /// Никнейм меняется отдельной операцией и только если он правда
  /// другой: занятым может оказаться лишь новый (specs/010-nicknames.md).
  Future<void> _save() => _change(() async {
    if (_nickname.text.trim() != _user.nickname) {
      await _api.setNickname(NicknameUpdate(nickname: _nickname.text));
    }
    return _api.updateMe(ProfileUpdate(name: _name.text, about: _about.text));
  }, 'Сохранено');

  Future<void> _pickAvatar() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery);
    if (picked == null) {
      return;
    }
    // Картинку уменьшает сервис: так аватар одинаков независимо от того,
    // с какого устройства его поставили (specs/002-profile.md, требование 7).
    final file = await MultipartFile.fromPath(
      'file',
      picked.path,
      filename: picked.name,
    );
    await _change(() => _api.setAvatar(file), 'Фотография обновлена');
  }

  Future<void> _removeAvatar() =>
      _change(() => _api.deleteAvatar(), 'Фотография убрана');

  Future<void> _signOut() async {
    setState(() => _busy = true);
    try {
      await AuthApi(apiClient(token: widget.token)).deleteSession();
    } on Exception catch (error) {
      // Сервис мог не ответить, но на этом устройстве человек уже вышел.
      debugPrint('$logMarker auth=sign_out_failed error=$error');
    }
    await widget.onSignedOut();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final hasAvatar = (_user.avatarUrl ?? '').isNotEmpty;
    final error = _error;
    final saved = _saved;

    return PopScope(
      // Наверх возвращается свежий профиль: и свой профиль, и шапка ленты
      // показывают никнейм и аватар и должны показывать те, что в сервисе.
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) {
          Navigator.of(context).pop(_user);
        }
      },
      child: AppScreen(
        title: 'Изменить профиль',
        // Свой профиль правят, а не проверяют связь: строка состояния
        // сервиса здесь только мешает.
        showServerStatus: false,
        child: ListView(
          children: [
            Center(
              child: UserAvatar(user: _user, radius: AvatarRadius.inProfile),
            ),
            const SizedBox(height: AppGap.medium),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                OutlinedButton.icon(
                  onPressed: _busy ? null : _pickAvatar,
                  icon: const Icon(Icons.photo_outlined),
                  label: Text(hasAvatar ? 'Сменить фото' : 'Выбрать фото'),
                ),
                if (hasAvatar) ...[
                  const SizedBox(width: AppGap.small),
                  TextButton(
                    onPressed: _busy ? null : _removeAvatar,
                    child: const Text('Убрать'),
                  ),
                ],
              ],
            ),
            const SizedBox(height: AppGap.large),
            TextField(
              controller: _nickname,
              enabled: !_busy,
              keyboardType: TextInputType.visiblePassword,
              autocorrect: false,
              enableSuggestions: false,
              inputFormatters: [LengthLimitingTextInputFormatter(20)],
              decoration: InputDecoration(
                labelText: 'Никнейм',
                helperText: 'Латиница, цифры и _, от 3 до 20 символов',
                errorText: _nicknameError,
                errorMaxLines: 2,
              ),
            ),
            const SizedBox(height: AppGap.medium),
            TextField(
              controller: _name,
              enabled: !_busy,
              textCapitalization: TextCapitalization.words,
              inputFormatters: [LengthLimitingTextInputFormatter(50)],
              decoration: const InputDecoration(labelText: 'Полное имя'),
            ),
            const SizedBox(height: AppGap.medium),
            TextField(
              controller: _about,
              enabled: !_busy,
              inputFormatters: [LengthLimitingTextInputFormatter(200)],
              decoration: const InputDecoration(labelText: 'О себе'),
            ),
            const SizedBox(height: AppGap.medium),
            FilledButton(
              onPressed: _busy ? null : _save,
              child: const Text('Сохранить'),
            ),
            if (error != null) ...[
              const SizedBox(height: AppGap.medium),
              // Повторить можно той же кнопкой «Сохранить» выше.
              ErrorView(message: error),
            ],
            if (saved != null) ...[
              const SizedBox(height: AppGap.medium),
              Text(saved, style: theme.textTheme.bodyMedium),
            ],
            const SizedBox(height: AppGap.large),
            const Divider(),
            const SizedBox(height: AppGap.medium),
            Text('Номер телефона', style: theme.textTheme.labelMedium),
            Text(_user.phone, style: theme.textTheme.bodyLarge),
            const SizedBox(height: AppGap.medium),
            Text('В МоейДаче с', style: theme.textTheme.labelMedium),
            Text(_date(_user.createdAt), style: theme.textTheme.bodyLarge),
            const SizedBox(height: AppGap.large),
            OutlinedButton(
              onPressed: _busy ? null : _signOut,
              child: const Text('Выйти'),
            ),
          ],
        ),
      ),
    );
  }

  static String _date(DateTime moment) {
    final local = moment.toLocal();
    return '${local.day.toString().padLeft(2, '0')}.'
        '${local.month.toString().padLeft(2, '0')}.${local.year}';
  }
}
