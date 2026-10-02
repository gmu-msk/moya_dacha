// Создать группу: specs/029-groups.md, требование 34.
//
// Название и описание — и всё: создаётся открытая группа по интересам.
// Геогруппы создаются сами, когда человек выбирает пункт в профиле.
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:moya_dacha_api/api.dart';

import '../api.dart';
import '../theme.dart';
import '../usage.dart';
import '../widgets/app_screen.dart';
import '../widgets/error_view.dart';

class NewGroupScreen extends StatefulWidget {
  const NewGroupScreen({super.key, required this.token});

  final String token;

  @override
  State<NewGroupScreen> createState() => _NewGroupScreenState();
}

class _NewGroupScreenState extends State<NewGroupScreen> {
  final _name = TextEditingController();
  final _description = TextEditingController();

  /// Ошибка названия и описания — под названием, остальные — под кнопкой.
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
    super.dispose();
  }

  Future<void> _create() async {
    setState(() {
      _busy = true;
      _fieldError = null;
      _error = null;
    });
    try {
      // Тип и правило шлём явно: сервер с main до #108 их требует.
      final group = await GroupsApi(apiClient(token: widget.token)).createGroup(
        GroupDraft(
          name: _name.text,
          description: _description.text,
          kind: 'interest',
          joinPolicy: 'open',
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
      setState(() {
        _busy = false;
        if (serviceErrorCode(error) == 'invalid_group') {
          _fieldError = errorMessage(error);
        } else {
          _error = errorMessage(error);
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final error = _error;

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
              hintText: 'Любители рыбалки, Розы и клематисы',
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
