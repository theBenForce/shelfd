import 'dart:async';
import 'package:audio_session/audio_session.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_web_plugins/url_strategy.dart';
import 'package:go_router/go_router.dart';
import 'package:just_audio_background/just_audio_background.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'data/services/storage_service.dart';
import 'data/utils/url_normalizer.dart';
import 'ui/core/theme.dart';
import 'ui/router.dart';
import 'ui/state/providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  if (kIsWeb) {
    usePathUrlStrategy();
  } else {
    try {
      final session = await AudioSession.instance;
      await session.configure(const AudioSessionConfiguration.speech());
    } catch (e) {
      debugPrint('AudioSession configuration failed: $e');
    }
    if (defaultTargetPlatform == TargetPlatform.android ||
        defaultTargetPlatform == TargetPlatform.iOS ||
        defaultTargetPlatform == TargetPlatform.macOS) {
      try {
        await JustAudioBackground.init(
          androidNotificationChannelId: 'com.justwriteapps.shelfd.channel.audio',
          androidNotificationChannelName: 'Shelfd Audio Playback',
          androidNotificationOngoing: true,
        );
      } catch (e) {
        debugPrint('JustAudioBackground.init failed: $e');
      }
    }
  }
  final prefs = await SharedPreferences.getInstance();
  final storage = StorageService(prefs);
  await storage.init();
  final token = storage.getAuthToken();
  final String initialRoute;
  if (token == null || token.isEmpty) {
    if (kIsWeb) {
      final path = Uri.base.path;
      final query = Uri.base.hasQuery ? '?${Uri.base.query}' : '';
      final fullLocation = '$path$query';
      final safeRedirect =
          UrlNormalizer.sanitizeSameOriginRedirect(fullLocation);
      if (safeRedirect != null &&
          safeRedirect != '/' &&
          safeRedirect != '/books') {
        initialRoute = '/connect?redirect=${Uri.encodeComponent(safeRedirect)}';
      } else {
        initialRoute = '/connect';
      }
    } else {
      initialRoute = '/connect';
    }
  } else if (kIsWeb) {
    final path = Uri.base.path;
    final query = Uri.base.hasQuery ? '?${Uri.base.query}' : '';
    final fullLocation = '$path$query';
    if (fullLocation.isEmpty ||
        fullLocation == '/' ||
        fullLocation == '/connect') {
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
        storageServiceProvider.overrideWithValue(storage),
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
  StreamSubscription<String>? _pairSubscription;

  @override
  void initState() {
    super.initState();
    final storage = ref.read(storageServiceProvider);
    final refreshListenable = ref.read(authRefreshListenableProvider);
    _router = createRouter(
      initialLocation: widget.initialRoute,
      storageService: storage,
      refreshListenable: refreshListenable,
    );

    Future.microtask(() async {
      await ref.read(authProvider.notifier).checkAuth();
      if (!mounted || kIsWeb) return;
      final qrService = ref.read(qrScannerServiceProvider);
      _pairSubscription = qrService.pairUriStream.listen((uri) {
        if (!mounted) return;
        ref.read(authProvider.notifier).pairWithQrUri(uri);
      });
      final initialPairUri = await qrService.getInitialPairUri();
      if (mounted && initialPairUri != null && initialPairUri.isNotEmpty) {
        await ref.read(authProvider.notifier).pairWithQrUri(initialPairUri);
      }
    });
  }

  @override
  void dispose() {
    _pairSubscription?.cancel();
    super.dispose();
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
