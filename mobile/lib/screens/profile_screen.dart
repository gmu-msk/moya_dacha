// Экран профиля: имя, «о себе» и аватар (specs/002-profile.md).
//
// Всё, что здесь видно, принадлежит владельцу токена: чужих профилей
// в приложении пока нет — автор появится вместе с лентой.
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' show MultipartFile;
import 'package:image_picker/image_picker.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../widgets/user_avatar.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key, required this.token, required this.user});

  final String token;
  final CurrentUser user;

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  late CurrentUser _user = widget.user;
  late final TextEditingController _name = TextEditingController(
    text: _user.name,
  );
  late final TextEditingController _about = TextEditingController(
    text: _user.about,
  );

  String? _error;
  String? _saved;
  bool _busy = false;

  ProfileApi get _api => ProfileApi(apiClient(token: widget.token));

  @override
  void dispose() {
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
      setState(() {
        _busy = false;
        _error = errorMessage(error);
      });
    }
  }

  Future<void> _save() => _change(
    () => _api.updateMe(ProfileUpdate(name: _name.text, about: _about.text)),
    'Сохранено',
  );

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

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final hasAvatar = (_user.avatarUrl ?? '').isNotEmpty;

    return PopScope(
      // Наверх возвращается свежий профиль: главный экран показывает имя
      // и аватар и должен показывать те, что сейчас в сервисе.
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) {
          Navigator.of(context).pop(_user);
        }
      },
      child: Scaffold(
        appBar: AppBar(title: const Text('Профиль')),
        body: SafeArea(
          child: ListView(
            padding: const EdgeInsets.all(24),
            children: [
              Center(child: UserAvatar(user: _user, radius: 56)),
              const SizedBox(height: 16),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  OutlinedButton.icon(
                    onPressed: _busy ? null : _pickAvatar,
                    icon: const Icon(Icons.photo_outlined),
                    label: Text(hasAvatar ? 'Сменить фото' : 'Выбрать фото'),
                  ),
                  if (hasAvatar) ...[
                    const SizedBox(width: 8),
                    TextButton(
                      onPressed: _busy ? null : _removeAvatar,
                      child: const Text('Убрать'),
                    ),
                  ],
                ],
              ),
              const SizedBox(height: 24),
              TextField(
                controller: _name,
                enabled: !_busy,
                textCapitalization: TextCapitalization.words,
                inputFormatters: [LengthLimitingTextInputFormatter(50)],
                decoration: const InputDecoration(
                  labelText: 'Имя',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _about,
                enabled: !_busy,
                inputFormatters: [LengthLimitingTextInputFormatter(200)],
                decoration: const InputDecoration(
                  labelText: 'О себе',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 16),
              FilledButton(
                onPressed: _busy ? null : _save,
                child: const Text('Сохранить'),
              ),
              if (_error != null) ...[
                const SizedBox(height: 16),
                Text(
                  _error!,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.error,
                  ),
                ),
              ],
              if (_saved != null) ...[
                const SizedBox(height: 16),
                Text(_saved!, style: theme.textTheme.bodyMedium),
              ],
              const SizedBox(height: 32),
              const Divider(),
              const SizedBox(height: 16),
              Text('Номер телефона', style: theme.textTheme.labelMedium),
              Text(_user.phone, style: theme.textTheme.bodyLarge),
              const SizedBox(height: 16),
              Text('В МоейДаче с', style: theme.textTheme.labelMedium),
              Text(_date(_user.createdAt), style: theme.textTheme.bodyLarge),
            ],
          ),
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
