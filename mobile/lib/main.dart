// Экран состояния сервиса — пока единственный экран приложения.
//
// Продуктовых эндпоинтов в контракте ещё нет, поэтому приложение показывает
// то единственное, что контракт умеет: отвечает ли сервис и жива ли база.
// Он же замыкает демо целиком — эмулятор -> приложение -> API -> Postgres,
// см. demo/README.md.
import 'package:flutter/material.dart';
import 'package:moya_dacha_api/api.dart';

/// Адрес API. По умолчанию — демо-стенд с точки зрения Android-эмулятора:
/// 10.0.2.2 это 127.0.0.1 машины-хоста. Переопределяется при сборке:
/// `flutter build apk --dart-define=API_BASE_URL=http://192.168.1.10:8080/api`.
const apiBaseUrl = String.fromEnvironment(
  'API_BASE_URL',
  defaultValue: 'http://10.0.2.2:8080/api',
);

/// Строка, по которой e2e-проверка в CI узнаёт исход в логах приложения.
const _logMarker = 'MOYA_DACHA_DEMO';

void main() {
  runApp(const MoyaDachaApp());
}

class MoyaDachaApp extends StatelessWidget {
  const MoyaDachaApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'МояДача',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF3F7D3F)),
      ),
      home: const StatusScreen(),
    );
  }
}

class StatusScreen extends StatefulWidget {
  const StatusScreen({super.key});

  @override
  State<StatusScreen> createState() => _StatusScreenState();
}

class _StatusScreenState extends State<StatusScreen> {
  late Future<Health?> _health;

  @override
  void initState() {
    super.initState();
    _health = _check();
  }

  Future<Health?> _check() async {
    final api = OperationsApi(ApiClient(basePath: apiBaseUrl));
    try {
      final health = await api.getHealth();
      debugPrint('$_logMarker health=${health?.status}');
      return health;
    } catch (error) {
      debugPrint('$_logMarker health=error error=$error');
      rethrow;
    }
  }

  void _retry() {
    setState(() {
      _health = _check();
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('МояДача')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: FutureBuilder<Health?>(
            future: _health,
            builder: (context, snapshot) {
              switch (snapshot.connectionState) {
                case ConnectionState.waiting:
                  return const CircularProgressIndicator();
                default:
                  return _Result(
                    ok: snapshot.hasData,
                    detail: snapshot.hasData
                        ? apiBaseUrl
                        : '$apiBaseUrl\n\n${snapshot.error}',
                    onRetry: _retry,
                  );
              }
            },
          ),
        ),
      ),
    );
  }
}

class _Result extends StatelessWidget {
  const _Result({
    required this.ok,
    required this.detail,
    required this.onRetry,
  });

  final bool ok;
  final String detail;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          ok ? Icons.check_circle_outline : Icons.cloud_off,
          size: 72,
          color: ok ? theme.colorScheme.primary : theme.colorScheme.error,
        ),
        const SizedBox(height: 16),
        Text(
          ok ? 'Сервер отвечает, база жива' : 'Сервер не отвечает',
          style: theme.textTheme.titleLarge,
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 12),
        Text(
          detail,
          style: theme.textTheme.bodySmall,
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 24),
        FilledButton(onPressed: onRetry, child: const Text('Проверить ещё раз')),
      ],
    );
  }
}
