import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'data/services/storage_service.dart';
import 'ui/core/theme.dart';
import 'ui/router.dart';
import 'ui/state/providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final prefs = await SharedPreferences.getInstance();
  final storage = StorageService(prefs);
  final token = storage.getAuthToken();
  final initialRoute = (token != null && token.isNotEmpty) ? '/library' : '/connect';

  runApp(
    ProviderScope(
      overrides: [
        sharedPreferencesProvider.overrideWithValue(prefs),
      ],
      child: ShelfApp(initialRoute: initialRoute),
    ),
  );
}

class ShelfApp extends ConsumerWidget {
  final String initialRoute;

  const ShelfApp({super.key, required this.initialRoute});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final readerSettings = ref.watch(readerSettingsProvider);
    final theme = AppTheme.buildTheme(readerSettings.themeMode);
    final router = createRouter(initialLocation: initialRoute);

    return MaterialApp.router(
      title: 'Shelf',
      debugShowCheckedModeBanner: false,
      theme: theme,
      routerConfig: router,
    );
  }
}
