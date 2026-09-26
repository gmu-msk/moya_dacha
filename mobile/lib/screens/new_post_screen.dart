// Экран создания поста (specs/003-posts.md).
//
// Вид «Сад» (2a): экран выезжает снизу и сразу предлагает последние фото
// из галереи телефона сеткой 3×N. Касание фото выбирает его (номер в
// кружке показывает порядок), повторное — снимает. Выбранные стоят лентой
// превью наверху. «Все фото» открывает системный выбор, если нужного
// снимка нет среди последних или доступ к галерее не дали.
//
// Фотографии уходят на сервис по одной, сразу после выбора: на дачной
// связи это разница между «одна не долетела, переотправим её» и
// «начинайте всё заново».
import 'dart:math' as math;
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:http/http.dart' show MultipartFile;
import 'package:image_picker/image_picker.dart';
import 'package:moya_dacha_api/api.dart';
import 'package:photo_manager/photo_manager.dart';

import '../api.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';
import '../widgets/visibility_picker.dart';

/// Превью выбранной фотографии: 104×130, как на макете.
const _selectedSize = Size(104, 130);

/// Сколько последних фото предлагается сразу.
const _recentCount = 60;

/// Сторона превью в сетке, в пикселях снимка (не экрана).
const _gridThumb = ThumbnailSize.square(300);

/// До какого размера фото уменьшается перед отправкой: столько же
/// оставит сервис (specs/003-posts.md, требование 9).
const _uploadSize = ThumbnailSize(1600, 1600);

const maxPostPhotos = 10;

/// Фотография в процессе: байты на устройстве, потом ещё и загруженное
/// медиа. Пока `uploaded` пусто, она в пост не годится.
class _Photo {
  _Photo({required this.key, required this.bytes, required this.name});

  /// Откуда взята: id снимка галереи или путь файла из системного выбора.
  final String key;
  final Uint8List bytes;
  final String name;
  Media? uploaded;
  String? error;
  bool busy = true;
}

class NewPostScreen extends StatefulWidget {
  const NewPostScreen({super.key, required this.token, this.closed = false});

  final String token;
  final bool closed;

  @override
  State<NewPostScreen> createState() => _NewPostScreenState();
}

class _NewPostScreenState extends State<NewPostScreen> {
  final List<_Photo> _photos = [];
  final TextEditingController _caption = TextEditingController();

  /// Последние снимки галереи; пусто — доступа нет или снимков нет.
  List<AssetEntity> _recent = const [];
  final Map<String, Future<Uint8List?>> _thumbs = {};
  bool _galleryLoading = true;

  String? _error;
  bool _publishing = false;
  int _shakes = 0;

  PostVisibility _visibility = PostVisibility.all;

  PostsApi get _api => PostsApi(apiClient(token: widget.token));

  bool get _ready =>
      !_publishing &&
      _photos.isNotEmpty &&
      _photos.every((photo) => photo.uploaded != null);

  @override
  void initState() {
    super.initState();
    usage.screen('new_post');
    _loadRecent();
  }

  @override
  void dispose() {
    _caption.dispose();
    super.dispose();
  }

  Future<void> _loadRecent() async {
    try {
      final permission = await PhotoManager.requestPermissionExtend();
      if (!permission.hasAccess) {
        debugPrint('$logMarker post=gallery_denied');
        if (mounted) {
          setState(() => _galleryLoading = false);
        }
        return;
      }
      final albums = await PhotoManager.getAssetPathList(
        type: RequestType.image,
        onlyAll: true,
      );
      final recent = albums.isEmpty
          ? <AssetEntity>[]
          : await albums.first.getAssetListPaged(page: 0, size: _recentCount);
      if (!mounted) {
        return;
      }
      setState(() {
        _recent = recent;
        _galleryLoading = false;
      });
    } on Exception catch (error) {
      debugPrint('$logMarker post=gallery_failed error=$error');
      if (mounted) {
        setState(() => _galleryLoading = false);
      }
    }
  }

  Future<Uint8List?> _thumb(AssetEntity asset) => _thumbs.putIfAbsent(
    asset.id,
    () => asset.thumbnailDataWithSize(_gridThumb, quality: 80),
  );

  _Photo? _selected(String key) {
    for (final photo in _photos) {
      if (photo.key == key) {
        return photo;
      }
    }
    return null;
  }

  void _full() {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('Не больше $maxPostPhotos фотографий')),
    );
  }

  /// Касание снимка в сетке: выбрать или снять выбор.
  Future<void> _toggle(AssetEntity asset) async {
    if (_publishing) {
      return;
    }
    final picked = _selected(asset.id);
    if (picked != null) {
      _remove(picked);
      return;
    }
    if (_photos.length >= maxPostPhotos) {
      _full();
      return;
    }
    final bytes = await asset.thumbnailDataWithSize(_uploadSize, quality: 85);
    if (bytes == null || !mounted) {
      return;
    }
    final photo = _Photo(
      key: asset.id,
      bytes: bytes,
      name: '${asset.id.replaceAll(RegExp(r'\W'), '_')}.jpg',
    );
    setState(() {
      _photos.add(photo);
      _error = null;
    });
    await _upload(photo);
  }

  /// «Все фото» — системный выбор, когда нужного снимка нет в сетке.
  Future<void> _pickSystem() async {
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
      added.add(
        _Photo(key: file.path, bytes: await file.readAsBytes(), name: file.name),
      );
    }
    if (!mounted) {
      return;
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
      final file = MultipartFile.fromBytes(
        'file',
        photo.bytes,
        filename: photo.name,
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
    if (_photos.isEmpty) {
      // Без фото поста не бывает: подсказка качается (правило 17).
      setState(() => _shakes++);
      return;
    }
    if (!_ready) {
      return;
    }
    setState(() {
      _publishing = true;
      _error = null;
    });

    try {
      final post = await _api.createPost(
        PostDraft(
          mediaIds: [for (final photo in _photos) photo.uploaded!.id],
          caption: _caption.text,
          visibility: _visibility,
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
    final scheme = theme.colorScheme;
    final error = _error;

    return AppScreen(
      title: 'Новый пост',
      showServerStatus: false,
      padded: false,
      child: Column(
        children: [
          Expanded(
            child: ListView(
              padding: const EdgeInsets.fromLTRB(
                AppGap.medium,
                AppGap.tiny,
                AppGap.medium,
                AppGap.medium,
              ),
              children: [
                AnimatedSize(
                  duration: AppMotion.standard,
                  curve: AppMotion.ease,
                  alignment: Alignment.topCenter,
                  child: _photos.isEmpty ? _emptyHint(theme) : _selectedStrip(),
                ),
                const SizedBox(height: AppGap.large),
                TextField(
                  controller: _caption,
                  enabled: !_publishing,
                  maxLines: 4,
                  minLines: 2,
                  maxLength: 1000,
                  decoration: const InputDecoration(
                    labelText: 'Подпись (необязательно)',
                    hintText: 'Что выросло?',
                    alignLabelWithHint: true,
                  ),
                ),
                const SizedBox(height: AppGap.small),
                VisibilityPicker(
                  value: _visibility,
                  closed: widget.closed,
                  enabled: !_publishing,
                  onChanged: (picked) => setState(() => _visibility = picked),
                ),
                const SizedBox(height: AppGap.large),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        'Выбрать фотографии',
                        style: theme.textTheme.titleMedium,
                      ),
                    ),
                    TextButton.icon(
                      onPressed: _photos.length >= maxPostPhotos || _publishing
                          ? null
                          : _pickSystem,
                      icon: const Icon(Icons.photo_library_outlined),
                      label: const Text('Все фото'),
                    ),
                  ],
                ),
                const SizedBox(height: AppGap.small),
                _grid(scheme),
                if (error != null) ...[
                  const SizedBox(height: AppGap.medium),
                  ErrorView(message: error, onRetry: _publish),
                ],
              ],
            ),
          ),
          DecoratedBox(
            decoration: BoxDecoration(
              border: Border(
                top: BorderSide(
                  color: scheme.outlineVariant,
                  width: AppShape.hairline,
                ),
              ),
            ),
            child: SafeArea(
              top: false,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(
                  AppGap.medium,
                  AppGap.small,
                  AppGap.medium,
                  AppGap.medium,
                ),
                child: SizedBox(
                  width: double.infinity,
                  child: FilledButton(
                    onPressed: _ready || _photos.isEmpty ? _publish : null,
                    style: _photos.isEmpty
                        ? FilledButton.styleFrom(
                            backgroundColor: scheme.surfaceContainerHighest,
                            foregroundColor: scheme.onSurfaceVariant,
                          )
                        : null,
                    child: Text(_publishing ? 'Публикую…' : 'Опубликовать'),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _emptyHint(ThemeData theme) {
    final scheme = theme.colorScheme;
    return TweenAnimationBuilder<double>(
      key: ValueKey(_shakes),
      tween: Tween(begin: _shakes == 0 ? 1.0 : 0.0, end: 1.0),
      duration: const Duration(milliseconds: 450),
      builder: (context, t, child) => Transform.translate(
        offset: Offset(math.sin(t * math.pi * 6) * (1 - t) * AppGap.small, 0),
        child: child,
      ),
      child: Container(
        padding: const EdgeInsets.all(AppGap.medium),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(AppShape.medium),
          border: Border.all(color: scheme.outlineVariant, width: AppShape.hairline),
        ),
        child: Row(
          children: [
            Icon(Icons.add_a_photo_outlined, color: scheme.primary, size: 30),
            const SizedBox(width: AppGap.snug),
            Expanded(
              child: Text(
                'Поста без фотографии не бывает: выберите от одной '
                'до десяти.',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _selectedStrip() {
    return SizedBox(
      height: _selectedSize.height,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: _photos.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppGap.small),
        itemBuilder: (_, index) => _selectedTile(_photos[index], index),
      ),
    );
  }

  Widget _selectedTile(_Photo photo, int index) {
    final scheme = Theme.of(context).colorScheme;

    return TweenAnimationBuilder<double>(
      key: ValueKey(photo.key),
      tween: Tween(begin: 0.85, end: 1),
      duration: AppMotion.standard,
      curve: AppMotion.spring,
      builder: (context, scale, child) =>
          Transform.scale(scale: scale, child: child),
      child: SizedBox(
        width: _selectedSize.width,
        height: _selectedSize.height,
        child: ClipRRect(
          borderRadius: BorderRadius.circular(AppShape.medium),
          child: Stack(
            fit: StackFit.expand,
            children: [
              Image.memory(photo.bytes, fit: BoxFit.cover),
              if (photo.busy)
                const ColoredBox(
                  color: Colors.black38,
                  child: Center(child: CircularProgressIndicator()),
                ),
              if (photo.error != null)
                ColoredBox(
                  color: scheme.error.withValues(alpha: 0.75),
                  child: Center(
                    child: IconButton(
                      tooltip: 'Отправить заново',
                      onPressed: () => _upload(photo),
                      icon: const Icon(Icons.refresh, color: Colors.white),
                    ),
                  ),
                ),
              Positioned(
                left: AppGap.small,
                top: AppGap.small,
                child: _Number(index + 1),
              ),
              Positioned(
                right: 0,
                top: 0,
                child: IconButton(
                  tooltip: 'Убрать',
                  onPressed: _publishing ? null : () => _remove(photo),
                  icon: const Icon(Icons.cancel, color: Colors.white),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _grid(ColorScheme scheme) {
    if (_galleryLoading) {
      return const Padding(
        padding: EdgeInsets.all(AppGap.large),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    if (_recent.isEmpty) {
      return Text(
        'Галерея телефона недоступна. Нажмите «Все фото», чтобы выбрать '
        'снимки.',
        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
          color: scheme.onSurfaceVariant,
        ),
      );
    }
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: _recent.length,
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        mainAxisSpacing: AppGap.tiny,
        crossAxisSpacing: AppGap.tiny,
      ),
      itemBuilder: (_, index) => _gridTile(_recent[index], scheme),
    );
  }

  Widget _gridTile(AssetEntity asset, ColorScheme scheme) {
    final picked = _selected(asset.id);
    final number = picked == null ? null : _photos.indexOf(picked) + 1;

    return Semantics(
      button: true,
      selected: picked != null,
      label: picked == null ? 'Выбрать фото' : 'Фото $number, снять выбор',
      child: GestureDetector(
        onTap: () => _toggle(asset),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(AppShape.small),
          child: Stack(
            fit: StackFit.expand,
            children: [
              ColoredBox(color: scheme.surfaceContainerHighest),
              AnimatedScale(
                scale: picked == null ? 1 : 0.88,
                duration: AppMotion.quick,
                curve: AppMotion.spring,
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(
                    picked == null ? 0 : AppShape.medium,
                  ),
                  child: FutureBuilder<Uint8List?>(
                    future: _thumb(asset),
                    builder: (_, snap) => snap.data == null
                        ? const SizedBox.shrink()
                        : Image.memory(snap.data!, fit: BoxFit.cover),
                  ),
                ),
              ),
              Positioned(
                right: AppGap.small,
                top: AppGap.small,
                child: number == null
                    ? const _EmptyCircle()
                    : _Number(number),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Номер выбранной фотографии в кружке основной краски.
class _Number extends StatelessWidget {
  const _Number(this.value);

  final int value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      width: 26,
      height: 26,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: theme.colorScheme.primary,
        shape: BoxShape.circle,
        border: Border.all(color: Colors.white, width: 2),
      ),
      child: Text(
        '$value',
        style: theme.textTheme.labelMedium?.copyWith(
          color: theme.colorScheme.onPrimary,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

/// Пустой кружок: фото можно выбрать.
class _EmptyCircle extends StatelessWidget {
  const _EmptyCircle();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 26,
      height: 26,
      decoration: BoxDecoration(
        color: Colors.black26,
        shape: BoxShape.circle,
        border: Border.all(color: Colors.white, width: 2),
      ),
    );
  }
}
