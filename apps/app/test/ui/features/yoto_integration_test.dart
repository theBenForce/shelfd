import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/yoto.dart';
import 'package:shelf/data/repositories/yoto_repository.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/book_detail/send_to_yoto_dialog.dart';
import 'package:shelf/ui/features/settings/settings_view.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('RFC 7636 PKCE S256 challenge matches standard test vector', () {
    // RFC 7636 Appendix B test vector:
    // Verifier: dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk
    // Challenge: E9Melhoa2OwvFrZpTJguCHaoeK1t8URWbuGJSstw-cM
    const verifier = 'dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk';
    final challenge = YotoRepository.computePkceS256Challenge(verifier);
    expect(challenge, 'E9Melhoa2OwvFrZpTJguCHaoeK1t8URWbuGJSstw-cM');
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
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

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
          yotoAccountProvider.overrideWith(_MockLinkedYotoNotifier.new),
          yotoPlaylistsProvider.overrideWith((ref) async => const [
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
              ]),
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
    await tester.tap(find.byKey(const Key('yoto_customize_chapter_icons_tile')));
    await tester.pumpAndSettle();

    expect(find.text('1. An Unexpected Party'), findsOneWidget);
    expect(find.text('2. Roast Mutton'), findsOneWidget);
    expect(find.byKey(const Key('yoto_chapter_icon_button_0')), findsOneWidget);
    expect(find.byKey(const Key('yoto_chapter_icon_button_1')), findsOneWidget);
  });

  testWidgets('SettingsView renders Yoto Player Integration card', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
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
  });
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
