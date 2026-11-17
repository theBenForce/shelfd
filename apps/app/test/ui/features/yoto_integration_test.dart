import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/yoto.dart';
import 'package:shelf/data/repositories/yoto_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/book_detail/send_to_yoto_dialog.dart';
import 'package:shelf/ui/features/settings/settings_view.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('RFC 7636 PKCE S256 challenge matches standard test vector', () {
    // Verifier: dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk
    // SHA-256 base64url (unpadded): E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM
    const verifier = 'dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk';
    final challenge = YotoRepository.computePkceS256Challenge(verifier);
    expect(challenge, 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM');
  });

  test('Yoto models serialize and deserialize accurately', () {
    final icon = YotoDisplayIcon.fromJson({
      'displayIconId': 'icon-1',
      'mediaId': 'media-abc',
      'title': 'Dragon',
      'url': 'https://yoto.io/dragon.png',
      'publicTags': ['fantasy', 'dragon'],
      'public': true,
    });
    expect(icon.yotoUri, 'yoto:#media-abc');
    expect(icon.publicTags, contains('dragon'));

    final req = SendToYotoRequest(
      mode: 'existing',
      cardId: 'card-123',
      includeCover: true,
      useCoverAsIcon: false,
      defaultIconId: 'yoto:#default-icon',
      chapterIcons: const {0: 'yoto:#ch0', 1: 'yoto:#ch1'},
    );
    final json = req.toJson();
    expect(json['mode'], 'existing');
    expect(json['card_id'], 'card-123');
    expect((json['chapter_icons'] as Map<String, dynamic>)['1'], 'yoto:#ch1');
  });

  testWidgets('SendToYotoDialog shows playlist selector and chapter icon customization when linked',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 1200);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final mockYotoRepo = _MockYotoRepository(prefs);

    const book = Book(
      id: 'audio-book-1',
      title: 'The Hobbit',
      authors: [Author(id: 'a1', name: 'J.R.R. Tolkien')],
      bookType: 'audiobook',
      durationSeconds: 7200.0,
      audioChapters: [
        AudioChapter(
          id: 'ch1',
          bookId: 'audio-book-1',
          chapterIndex: 1,
          title: 'An Unexpected Party',
          startOffsetSec: 0,
          durationSec: 3600,
        ),
        AudioChapter(
          id: 'ch2',
          bookId: 'audio-book-1',
          chapterIndex: 2,
          title: 'Roast Mutton',
          startOffsetSec: 3600,
          durationSec: 3600,
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          yotoRepositoryProvider.overrideWithValue(mockYotoRepo),
          yotoAccountProvider.overrideWith(_MockLinkedYotoNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(
            body: SendToYotoDialog(book: book),
          ),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.text('Send to Yoto Playlist'), findsOneWidget);
    expect(find.text('Bedtime Stories'), findsOneWidget);
    expect(find.text('Road Trip Adventures'), findsOneWidget);
    expect(find.byKey(const Key('yoto_change_default_icon_button')), findsOneWidget);
    expect(find.byKey(const Key('yoto_use_cover_as_icon_switch')), findsOneWidget);

    // Expand per-chapter icon customization
    final customizeTile = find.byKey(const Key('yoto_customize_chapter_icons_tile'));
    await tester.ensureVisible(customizeTile);
    await tester.pumpAndSettle();
    await tester.tap(customizeTile);
    await tester.pumpAndSettle();

    expect(find.text('1. An Unexpected Party'), findsOneWidget);
    expect(find.text('2. Roast Mutton'), findsOneWidget);
    expect(find.byKey(const Key('yoto_chapter_icon_button_0')), findsOneWidget);
    expect(find.byKey(const Key('yoto_chapter_icon_button_1')), findsOneWidget);
  });

  testWidgets('SettingsView renders Yoto Player Integration card', (tester) async {
    tester.view.physicalSize = const Size(1200, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          yotoAccountProvider.overrideWith(_MockUnlinkedYotoNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.byKey(const Key('yoto_integration_settings_card')), findsOneWidget);
    expect(find.text('Yoto Player Integration'), findsOneWidget);
    expect(find.byKey(const Key('yoto_client_id_field')), findsOneWidget);
    expect(find.byKey(const Key('link_yoto_oauth_button')), findsOneWidget);

    // Tapping Authorize with Yoto (PKCE) computes S256 challenge and saves verifier
    await tester.ensureVisible(find.byKey(const Key('link_yoto_oauth_button')));
    await tester.tap(find.byKey(const Key('link_yoto_oauth_button')));
    await tester.pumpAndSettle();

    expect(prefs.getString('shelfd_yoto_client_id'), 'default-client-id');
    expect(prefs.getString('shelfd_yoto_pkce_verifier'), isNotNull);
    expect(prefs.getString('shelfd_yoto_pkce_verifier')!.length, 64);
  });

  testWidgets('SettingsView shows Reconnect / Sign In button when Yoto status fails with 401', (tester) async {
    tester.view.physicalSize = const Size(1200, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({
      'shelfd_auth_token': 'expired-token',
    });
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          yotoAccountProvider.overrideWith(_MockUnauthorizedYotoNotifier.new),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const SettingsView(),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.byKey(const Key('yoto_reconnect_signin_button')), findsOneWidget);
  });
}

class _MockYotoRepository extends YotoRepository {
  _MockYotoRepository(SharedPreferences prefs)
      : super(
          apiService: ApiService(baseUrl: 'http://localhost:8080'),
          storageService: StorageService(prefs),
        );

  @override
  Future<List<YotoPlaylist>> getPlaylists() async {
    return const [
      YotoPlaylist(
        cardId: 'card-bedtime',
        title: 'Bedtime Stories',
        chapterCount: 4,
      ),
      YotoPlaylist(
        cardId: 'card-roadtrip',
        title: 'Road Trip Adventures',
        chapterCount: 2,
      ),
    ];
  }
}

class _MockLinkedYotoNotifier extends YotoAccountNotifier {
  @override
  YotoAccountState build() {
    return const YotoAccountState(
      status: YotoStatus(
        isLinked: true,
        clientId: 'yoto-client-123',
      ),
      isLoading: false,
    );
  }
}

class _MockUnlinkedYotoNotifier extends YotoAccountNotifier {
  @override
  YotoAccountState build() {
    return const YotoAccountState(
      status: YotoStatus(
        isLinked: false,
        defaultClientId: 'default-client-id',
      ),
      isLoading: false,
    );
  }
}

class _MockUnauthorizedYotoNotifier extends YotoAccountNotifier {
  @override
  YotoAccountState build() {
    return const YotoAccountState(
      status: YotoStatus(isLinked: false),
      isLoading: false,
      errorMessage: 'Failed to load Yoto account status: ApiException(401): {"error":"Invalid or expired authorization token"}',
    );
  }
}
