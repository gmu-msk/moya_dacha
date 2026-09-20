// Экран создания поста (specs/003-posts.md).
//
// Фотографии уходят на сервис по одной, сразу после выбора: на дачной
// связи это разница между «одна не долетела, переотправим её» и
// «начинайте всё заново».
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:http/http.dart' show MultipartFile;
import 'package:image_picker/image_picker.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';

/// Сторона квадратного превью выбранной фотографии.
const _thumbnailSize = 104.0;

/// Сколько фотографий помещается в пост. То же число, что и на сервисе
/// (docs/adr/0006-post-is-media.md).
const maxPostPhotos = 4;

/// Фотография в процессе: сначала файл на устройстве, потом ещё и
/// загруженное медиа. Пока `uploaded` пусто, она в пост не годится.
class _Photo {
  _Photo(this.file);

  final XFile file;
  Media? uploaded;
  String? error;
  bool busy = true;
}

class NewPostScreen extends StatefulWidget {
  const NewPostScreen({super.key, required this.token});

  final String token;

  @override
  State<NewPostScreen> createState() => _NewPostScreenState();
}

class _NewPostScreenState extends State<NewPostScreen> {
  final List<_Photo> _photos = [];
  final TextEditingController _caption = TextEditingController();

  String? _error;
  bool _publishing = false;

  PostsApi get _api => PostsApi(apiClient(token: widget.token));

  bool get _ready =>
      !_publishing &&
      _photos.isNotEmpty &&
      _photos.every((photo) => photo.uploaded != null);

  @override
  void dispose() {
    _caption.dispose();
    super.dispose();
  }

  Future<void> _pick() async {
    // Уменьшаем перед отправкой до того же размера, до которого уменьшит
    // сервис: мегабайты по дачному интернету гонять незачем. Сервис всё
    // равно нормализует то, что пришло (specs/003-posts.md, требование 9).
    final picked = await ImagePicker().pickMultiImage(
      limit: maxPostPhotos - _photos.length,
      maxWidth: 1600,
      maxHeight: 1600,
      imageQuality: 85,
    );
    if (picked.isEmpty) {
      return;
    }

    final added = <_Photo>[];
    for (final file in picked.take(maxPostPhotos - _photos.length)) {
      added.add(_Photo(file));
    }
    setState(() {
      _photos.addAll(added);
      _error = null;
    });

    for (final photo in added) {
      await _upload(photo);
    }
  }

  Future<void> _upload(_Photo photo) async {
    if (!mounted) {
      return;
    }
    setState(() {
      photo.busy = true;
      photo.error = null;
    });

    try {
      final file = await MultipartFile.fromPath(
        'file',
        photo.file.path,
        filename: photo.file.name,
      );
      final uploaded = await _api.uploadMedia(file);
      debugPrint('$logMarker post=photo_uploaded id=${uploaded?.id}');
      if (!mounted) {
        return;
      }
      setState(() {
        photo.uploaded = uploaded;
        photo.busy = false;
      });
    } on Exception catch (error) {
      debugPrint('$logMarker post=photo_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        photo.busy = false;
        photo.error = errorMessage(error);
      });
    }
  }

  void _remove(_Photo photo) {
    setState(() => _photos.remove(photo));
  }

  Future<void> _publish() async {
    setState(() {
      _publishing = true;
      _error = null;
    });

    try {
      final post = await _api.createPost(
        PostDraft(
          mediaIds: [for (final photo in _photos) photo.uploaded!.id],
          caption: _caption.text,
        ),
      );
      debugPrint('$logMarker post=published id=${post?.id}');
      if (post == null) {
        throw ApiException(201, 'Сервис не вернул пост');
      }
      if (!mounted) {
        return;
      }
      Navigator.of(context).pop(post);
    } on Exception catch (error) {
      debugPrint('$logMarker post=publish_failed error=$error');
      if (!mounted) {
        return;
      }
      setState(() {
        _publishing = false;
        _error = errorMessage(error);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final error = _error;

    return AppScreen(
      title: 'Новый пост',
      // Здесь публикуют, а не проверяют связь: строка состояния сервиса
      // только отнимает место у фотографий.
      showServerStatus: false,
      child: ListView(
        children: [
          if (_photos.isEmpty)
            Text(
              'Поста без фотографии не бывает: выберите от одной '
              'до четырёх.',
              style: theme.textTheme.bodyMedium,
            )
          else
            Wrap(
              spacing: AppGap.small,
              runSpacing: AppGap.small,
              children: [for (final photo in _photos) _thumbnail(photo)],
            ),
          const SizedBox(height: AppGap.medium),
          OutlinedButton.icon(
            onPressed: _photos.length >= maxPostPhotos || _publishing
                ? null
                : _pick,
            icon: const Icon(Icons.add_photo_alternate_outlined),
            label: Text(
              _photos.isEmpty ? 'Выбрать фотографии' : 'Добавить ещё',
            ),
          ),
          const SizedBox(height: AppGap.large),
          TextField(
            controller: _caption,
            enabled: !_publishing,
            maxLines: 4,
            maxLength: 1000,
            decoration: const InputDecoration(
              labelText: 'Подпись (необязательно)',
              alignLabelWithHint: true,
            ),
          ),
          if (error != null) ...[
            const SizedBox(height: AppGap.small),
            ErrorView(message: error, onRetry: _publish),
          ],
          const SizedBox(height: AppGap.medium),
          FilledButton(
            onPressed: _ready ? _publish : null,
            child: const Text('Опубликовать'),
          ),
        ],
      ),
    );
  }

  Widget _thumbnail(_Photo photo) {
    final theme = Theme.of(context);

    return SizedBox(
      width: _thumbnailSize,
      height: _thumbnailSize,
      child: Stack(
        fit: StackFit.expand,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(AppGap.small),
            child: Image.file(File(photo.file.path), fit: BoxFit.cover),
          ),
          if (photo.busy)
            Container(
              color: Colors.black38,
              alignment: Alignment.center,
              child: const CircularProgressIndicator(),
            ),
          if (photo.error != null)
            Container(
              color: theme.colorScheme.error.withValues(alpha: 0.75),
              alignment: Alignment.center,
              child: IconButton(
                tooltip: 'Отправить заново',
                onPressed: () => _upload(photo),
                icon: const Icon(Icons.refresh, color: Colors.white),
              ),
            ),
          Align(
            alignment: Alignment.topRight,
            child: IconButton(
              tooltip: 'Убрать',
              onPressed: _publishing ? null : () => _remove(photo),
              icon: const Icon(Icons.cancel, color: Colors.white),
            ),
          ),
        ],
      ),
    );
  }
}
