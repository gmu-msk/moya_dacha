// Населённый пункт: строка в профиле и поле выбора из подсказок
// справочника ФИАС (specs/025-places.md).
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';

/// Пункт в профиле: название, под ним мелко район и область
/// (требование 19). Подпись нужна, чтобы одноимённые пункты не путались.
class PlaceLine extends StatelessWidget {
  const PlaceLine({super.key, required this.place});

  final Place place;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(top: 2, right: AppGap.small),
          child: Icon(
            Icons.place_outlined,
            size: 20,
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        Expanded(child: PlaceText(place: place)),
      ],
    );
  }
}

/// Название и подпись пункта в две строки; пустая подпись — одна строка.
class PlaceText extends StatelessWidget {
  const PlaceText({super.key, required this.place});

  final Place place;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(place.name, style: theme.textTheme.bodyLarge),
        if (place.area.isNotEmpty)
          Text(
            place.area,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
      ],
    );
  }
}

/// Поле «Населённый пункт» на экране «Изменить профиль» (требования
/// 15–18). Выбранный пункт поднимается наверх через [onChanged]; набранный
/// и не выбранный текст пунктом не становится.
class PlaceField extends StatefulWidget {
  const PlaceField({
    super.key,
    required this.token,
    required this.place,
    required this.onChanged,
    this.enabled = true,
    this.suggest,
  });

  final String token;
  final Place? place;
  final ValueChanged<Place?> onChanged;
  final bool enabled;

  /// Откуда брать подсказки; по умолчанию — `GET /places`. Подменяется
  /// в тестах экрана.
  final Future<List<Place>> Function(String query)? suggest;

  @override
  State<PlaceField> createState() => _PlaceFieldState();
}

class _PlaceFieldState extends State<PlaceField> {
  static const _pause = Duration(milliseconds: 300);
  static const _minQuery = 2;

  final _query = TextEditingController();
  Timer? _timer;

  /// Номер запроса: ответ на устаревший набор не перетирает свежий.
  int _asked = 0;
  List<Place>? _places;
  String? _message;
  bool _loading = false;

  @override
  void dispose() {
    _timer?.cancel();
    _query.dispose();
    super.dispose();
  }

  void _onTyped(String text) {
    _timer?.cancel();
    if (text.trim().length < _minQuery) {
      setState(() {
        _asked++;
        _places = null;
        _message = null;
        _loading = false;
      });
      return;
    }
    _timer = Timer(_pause, () => _ask(text.trim()));
  }

  Future<void> _ask(String query) async {
    final asked = ++_asked;
    setState(() => _loading = true);
    try {
      final places = await (widget.suggest ?? _fromApi)(query);
      if (!mounted || asked != _asked) {
        return;
      }
      setState(() {
        _loading = false;
        _places = places;
        _message = places.isEmpty ? 'Ничего не нашлось' : null;
      });
    } on Exception catch (error) {
      debugPrint('$logMarker places=failed error=$error');
      if (!mounted || asked != _asked) {
        return;
      }
      setState(() {
        _loading = false;
        _places = null;
        _message = 'Подсказки сейчас недоступны';
      });
    }
  }

  Future<List<Place>> _fromApi(String query) async {
    final list = await ProfileApi(apiClient(token: widget.token))
        .getPlaces(query);
    return list?.places ?? const <Place>[];
  }

  void _pick(Place place) {
    _timer?.cancel();
    _query.clear();
    setState(() {
      _asked++;
      _places = null;
      _message = null;
      _loading = false;
    });
    widget.onChanged(place);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final place = widget.place;

    if (place != null) {
      return InputDecorator(
        decoration: const InputDecoration(labelText: 'Населённый пункт'),
        child: Row(
          children: [
            Expanded(child: PlaceText(place: place)),
            TextButton(
              onPressed: widget.enabled ? () => widget.onChanged(null) : null,
              child: const Text('Убрать'),
            ),
          ],
        ),
      );
    }

    final places = _places;
    final message = _message;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        TextField(
          controller: _query,
          enabled: widget.enabled,
          onChanged: _onTyped,
          textCapitalization: TextCapitalization.sentences,
          decoration: InputDecoration(
            labelText: 'Населённый пункт',
            helperText: 'СНТ, деревня, посёлок — выберите из подсказок',
            suffixIcon: _loading
                ? const Padding(
                    padding: EdgeInsets.all(AppGap.snug),
                    child: SizedBox.square(
                      dimension: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                  )
                : null,
          ),
        ),
        if (places != null && places.isNotEmpty)
          for (final p in places)
            ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(p.name),
              subtitle: p.area.isEmpty ? null : Text(p.area),
              onTap: widget.enabled ? () => _pick(p) : null,
            ),
        if (message != null)
          Padding(
            padding: const EdgeInsets.only(top: AppGap.small),
            child: Text(
              message,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
      ],
    );
  }
}
