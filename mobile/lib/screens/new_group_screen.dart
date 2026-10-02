// Создать группу: specs/029-groups.md, требования 1–6 и 31.
//
// Название, описание, тип; у типа «По месту» — место (подставлено из
// профиля) и радиус. Правило вступления. Созданная группа уходит наверх.
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';
import '../widgets/place_field.dart';

/// Правила вступления: значение, название и пояснение.
const _policies = <(String, String, String)>[
  ('open', 'Открытая', 'Вступает любой, сразу'),
  ('request', 'По заявке', 'Вы принимаете или отклоняете заявки'),
  ('invite', 'По приглашению', 'Только те, кого вы позвали; другим не видна'),
];

class NewGroupScreen extends StatefulWidget {
  const NewGroupScreen({super.key, required this.token, this.place});

  final String token;

  /// Пункт из профиля — место новой группы по месту.
  final Place? place;

  @override
  State<NewGroupScreen> createState() => _NewGroupScreenState();
}

class _NewGroupScreenState extends State<NewGroupScreen> {
  final _name = TextEditingController();
  final _description = TextEditingController();
  final _radius = TextEditingController();

  String _kind = 'interest';
  String _policy = 'open';
  late Place? _place = widget.place;

  /// Ошибка места — под его полем, ошибка названия и прочего — под
  /// названием, остальные — под кнопкой.
  String? _placeError;
  String? _fieldError;
  String? _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    usage.screen('group_new');
  }

  @override
  void dispose() {
    _name.dispose();
    _description.dispose();
    _radius.dispose();
    super.dispose();
  }

  Future<void> _create() async {
    final place = _kind == 'place';
    final radiusText = _radius.text.trim();
    setState(() {
      _busy = true;
      _placeError = null;
      _fieldError = null;
      _error = null;
    });
    try {
      final group = await GroupsApi(apiClient(token: widget.token)).createGroup(
        GroupDraft(
          name: _name.text,
          description: _description.text,
          kind: _kind,
          joinPolicy: _policy,
          placeId: place ? _place?.id : null,
          radiusKm: place && radiusText.isNotEmpty
              ? int.tryParse(radiusText) ?? 0
              : null,
        ),
      );
      debugPrint('$logMarker group=created id=${group?.id}');
      if (!mounted || group == null) {
        return;
      }
      Navigator.of(context).pop(group);
    } on Exception catch (error) {
      debugPrint('$logMarker group=create_failed error=$error');
      if (!mounted) {
        return;
      }
      final code = serviceErrorCode(error);
      setState(() {
        _busy = false;
        if (code == 'place_required' || code == 'unknown_place') {
          _placeError = errorMessage(error);
        } else if (code == 'invalid_group') {
          _fieldError = errorMessage(error);
        } else {
          _error = errorMessage(error);
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final error = _error;
    final placeError = _placeError;

    return AppScreen(
      title: 'Новая группа',
      showServerStatus: false,
      child: ListView(
        children: [
          TextField(
            controller: _name,
            enabled: !_busy,
            textCapitalization: TextCapitalization.sentences,
            inputFormatters: [LengthLimitingTextInputFormatter(60)],
            decoration: InputDecoration(
              labelText: 'Название',
              hintText: 'СНТ Ромашка, Любители рыбалки',
              errorText: _fieldError,
              errorMaxLines: 3,
            ),
          ),
          const SizedBox(height: AppGap.medium),
          TextField(
            controller: _description,
            enabled: !_busy,
            minLines: 2,
            maxLines: 5,
            textCapitalization: TextCapitalization.sentences,
            inputFormatters: [LengthLimitingTextInputFormatter(500)],
            decoration: const InputDecoration(
              labelText: 'Описание',
              hintText: 'О чём группа и кого в ней ждут',
            ),
          ),
          const SizedBox(height: AppGap.large),
          Text('Тип', style: theme.textTheme.titleSmall),
          const SizedBox(height: AppGap.small),
          SegmentedButton<String>(
            segments: const [
              ButtonSegment(value: 'interest', label: Text('По интересам')),
              ButtonSegment(value: 'place', label: Text('По месту')),
            ],
            selected: {_kind},
            onSelectionChanged: _busy
                ? null
                : (value) => setState(() => _kind = value.first),
          ),
          if (_kind == 'place') ...[
            const SizedBox(height: AppGap.medium),
            PlaceField(
              token: widget.token,
              place: _place,
              enabled: !_busy,
              label: 'Место',
              helper: 'СНТ, деревня, посёлок — выберите из подсказок',
              onChanged: (place) => setState(() => _place = place),
            ),
            if (placeError != null)
              Padding(
                padding: const EdgeInsets.only(top: AppGap.tiny),
                child: Text(
                  placeError,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.error,
                  ),
                ),
              ),
            const SizedBox(height: AppGap.medium),
            TextField(
              controller: _radius,
              enabled: !_busy,
              keyboardType: TextInputType.number,
              inputFormatters: [
                FilteringTextInputFormatter.digitsOnly,
                LengthLimitingTextInputFormatter(3),
              ],
              decoration: const InputDecoration(
                labelText: 'Радиус, км',
                helperText:
                    'Необязательно: соседи в стольких километрах '
                    'вокруг тоже «рядом»',
                helperMaxLines: 2,
              ),
            ),
          ],
          const SizedBox(height: AppGap.large),
          Text('Вступление', style: theme.textTheme.titleSmall),
          RadioGroup<String>(
            groupValue: _policy,
            onChanged: (value) {
              if (!_busy && value != null) {
                setState(() => _policy = value);
              }
            },
            child: Column(
              children: [
                for (final (value, title, hint) in _policies)
                  RadioListTile<String>(
                    contentPadding: EdgeInsets.zero,
                    value: value,
                    title: Text(title),
                    subtitle: Text(hint),
                  ),
              ],
            ),
          ),
          const SizedBox(height: AppGap.medium),
          FilledButton(
            onPressed: _busy ? null : _create,
            child: const Text('Создать группу'),
          ),
          if (error != null) ...[
            const SizedBox(height: AppGap.medium),
            ErrorView(message: error),
          ],
        ],
      ),
    );
  }
}
