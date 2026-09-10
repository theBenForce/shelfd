import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_web_plugins/url_strategy.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'data/services/storage_service.dart';
import 'ui/core/theme.dart';
import 'ui/router.dart';
import 'ui/state/providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  if (kIsWeb) {
    usePathUrlStrategy();
  }
  final prefs = await SharedPreferences.getInstance();
  final storage = StorageService(prefs);
  final token = storage.getAuthToken();
  final String initialRoute;
  if (token == null || token.isEmpty) {
    initialRoute = '/connect';
  } else if (kIsWeb) {
    final path = Uri.base.path;
    final query = Uri.base.hasQuery ? '?${Uri.base.query}' : '';
    final fullLocation = '$path$query';
    if (fullLocation.isEmpty || fullLocation == '/' || fullLocation == '/connect') {
      initialRoute = '/books';
    } else {
      initialRoute = fullLocation;
    }
  } else {
    initialRoute = '/books';
  }

  runApp(
    ProviderScope(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
      ],
      child: ShelfApp(initialRoute: initialRoute),
    ),
  );
}

class ShelfApp extends ConsumerStatefulWidget {
  final String initialRoute;

  const ShelfApp({super.key, required this.initialRoute});

  @override
  ConsumerState<ShelfApp> createState() => _ShelfAppState();
}

class _ShelfAppState extends ConsumerState<ShelfApp> {
  late final GoRouter _router;

  @override
  void initState() {
    super.initState();
    final storage = ref.read(storageServiceProvider);
    _router = createRouter(initialLocation: widget.initialRoute, storageService: storage);
  }

  @override
  Widget build(BuildContext context) {
    final readerSettings = ref.watch(readerSettingsProvider);
    final theme = AppTheme.buildTheme(readerSettings.themeMode);

    return MaterialApp.router(
      title: 'Shelfd',
      debugShowCheckedModeBanner: false,
      theme: theme,
      routerConfig: _router,
    );
  }
}
