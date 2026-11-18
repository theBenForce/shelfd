import 'dart:async';
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
import 'package:shelf/ui/core/shared_layout.dart';
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

    final iconWithoutHash = YotoDisplayIcon.fromJson({
      'displayIconId': 'icon-2',
      'mediaId': 'AC6ySygbay79fIMHmMtYN9ekBo1kwMvRoi4DQW_CZW0',
      'yotoUri': 'yoto:AC6ySygbay79fIMHmMtYN9ekBo1kwMvRoi4DQW_CZW0',
      'title': 'Book',
      'url': 'https://yoto.io/book.png',
    });
    expect(
      iconWithoutHash.yotoUri,
      'yoto:#AC6ySygbay79fIMHmMtYN9ekBo1kwMvRoi4DQW_CZW0',
    );

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
    expect(find.text('4 chapters'), findsOneWidget);
    expect(find.text('Road Trip Adventures'), findsOneWidget);
    expect(find.text('2 chapters'), findsOneWidget);
    expect(find.byKey(const Key('yoto_existing_action_selector')), findsOneWidget);
    expect(find.text('Append Audio'), findsOneWidget);
    expect(find.text('Replace Audio'), findsOneWidget);
    expect(find.text('Append to Playlist'), findsOneWidget);
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

    // Switching to Replace Audio updates button label to Replace Playlist
    await tester.ensureVisible(find.text('Replace Audio'));
    await tester.tap(find.text('Replace Audio'));
    await tester.pumpAndSettle();
    expect(find.text('Replace Playlist'), findsOneWidget);

    // Switching back to Append Audio updates button label to Append to Playlist
    await tester.tap(find.text('Append Audio'));
    await tester.pumpAndSettle();
    expect(find.text('Append to Playlist'), findsOneWidget);

    // Submitting in default append mode sends mode='append' and preserves existing cover (includeCover=false)
    await tester.tap(find.byKey(const Key('confirm_send_to_yoto_button')));
    await tester.pumpAndSettle();

    expect(mockYotoRepo.lastSentBookId, 'audio-book-1');
    expect(mockYotoRepo.lastRequest, isNotNull);
    expect(mockYotoRepo.lastRequest!.mode, 'append');
    expect(mockYotoRepo.lastRequest!.cardId, 'card-bedtime');
    expect(mockYotoRepo.lastRequest!.includeCover, isFalse);
  });

  testWidgets('SendToYotoDialog.series sends entire series audio in sequence order',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 1200);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final mockYotoRepo = _MockYotoRepository(prefs);

    const seriesBooks = [
      Book(
        id: 'series-book-1',
        title: 'The Hobbit',
        bookType: 'audiobook',
        durationSeconds: 3600.0,
        audioChapters: [
          AudioChapter(
            id: 'ch1',
            bookId: 'series-book-1',
            chapterIndex: 1,
            title: 'An Unexpected Party',
            startOffsetSec: 0,
            durationSec: 1800,
          ),
          AudioChapter(
            id: 'ch2',
            bookId: 'series-book-1',
            chapterIndex: 2,
            title: 'Roast Mutton',
            startOffsetSec: 1800,
            durationSec: 1800,
          ),
        ],
      ),
      Book(
        id: 'series-book-2',
        title: 'The Fellowship of the Ring',
        bookType: 'audiobook',
        durationSeconds: 3000.0,
        audioChapters: [
          AudioChapter(
            id: 'ch3',
            bookId: 'series-book-2',
            chapterIndex: 1,
            title: 'Chapter 1',
            startOffsetSec: 0,
            durationSec: 3000,
          ),
        ],
      ),
    ];

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
            body: SendToYotoDialog.series(
              seriesId: 'series-middle-earth',
              seriesName: 'Middle-earth',
              seriesBooks: seriesBooks,
            ),
          ),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.text('Send Series to Yoto Playlist'), findsOneWidget);
    expect(find.text('Middle-earth • 2 books • 3 chapters'), findsOneWidget);

    // Expand per-chapter icon customization to verify series chapter labels
    final customizeTile = find.byKey(const Key('yoto_customize_chapter_icons_tile'));
    await tester.ensureVisible(customizeTile);
    await tester.pumpAndSettle();
    await tester.tap(customizeTile);
    await tester.pumpAndSettle();

    expect(find.text('1. The Hobbit - An Unexpected Party'), findsOneWidget);
    expect(find.text('2. The Hobbit - Roast Mutton'), findsOneWidget);
    expect(find.text('3. The Fellowship of the Ring'), findsOneWidget);

    // Switch to Create New Playlist and submit
    await tester.ensureVisible(find.text('Create New Playlist'));
    await tester.tap(find.text('Create New Playlist'));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('confirm_send_to_yoto_button')));
    await tester.pumpAndSettle();

    expect(mockYotoRepo.lastSentSeriesId, 'series-middle-earth');
    expect(mockYotoRepo.lastRequest, isNotNull);
    expect(mockYotoRepo.lastRequest!.mode, 'new');
    expect(mockYotoRepo.lastRequest!.newPlaylistTitle, 'Middle-earth');
    expect(mockYotoRepo.lastRequest!.bookIds, ['series-book-1', 'series-book-2']);
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

  testWidgets(
      'YotoUploadQueueBanner shows current & total progress bars and toast notification on error',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final mockYotoRepo = _MockYotoRepository(prefs);

    final firstUploadCompleter = Completer<SendToYotoResult>();
    final secondUploadCompleter = Completer<SendToYotoResult>();
    mockYotoRepo.bookUploadCompleters['book-1'] = firstUploadCompleter;
    mockYotoRepo.bookUploadCompleters['book-2'] = secondUploadCompleter;
    mockYotoRepo.simulatedProgress = const YotoServerUploadProgress(
      isActive: true,
      stage: 'uploading',
      currentProgress: 0.5,
      overallProgress: 0.5,
      completedItems: 0,
      totalItems: 1,
    );

    late WidgetRef capturedRef;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          yotoRepositoryProvider.overrideWithValue(mockYotoRepo),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Scaffold(
            body: Consumer(
              builder: (context, ref, _) {
                capturedRef = ref;
                return const Column(
                  children: [
                    Expanded(child: SizedBox()),
                    YotoUploadQueueBanner(),
                  ],
                );
              },
            ),
          ),
        ),
      ),
    );

    // Initially hidden when queue is empty
    expect(find.byKey(const Key('yoto_upload_queue_banner')), findsNothing);

    // Enqueue 2 books into background upload queue
    final notifier = capturedRef.read(yotoUploadQueueProvider.notifier);
    notifier.enqueueBookUpload(
      bookId: 'book-1',
      title: 'The Hobbit',
      request: const SendToYotoRequest(mode: 'append', cardId: 'card-bedtime'),
    );
    notifier.enqueueBookUpload(
      bookId: 'book-2',
      title: 'The Fellowship of the Ring',
      request: const SendToYotoRequest(mode: 'append', cardId: 'card-bedtime'),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    // Banner and both progress bars are visible
    expect(find.byKey(const Key('yoto_upload_queue_banner')), findsOneWidget);
    expect(find.byKey(const Key('yoto_current_upload_progress_bar')), findsOneWidget);
    expect(find.byKey(const Key('yoto_total_uploads_progress_bar')), findsOneWidget);
    expect(find.textContaining('Current Upload: The Hobbit'), findsOneWidget);
    expect(find.text('Total Uploads (0 of 2)'), findsOneWidget);

    final currentBar1 = tester.widget<LinearProgressIndicator>(
      find.byKey(const Key('yoto_current_upload_progress_bar')),
    );
    final totalBar1 = tester.widget<LinearProgressIndicator>(
      find.byKey(const Key('yoto_total_uploads_progress_bar')),
    );
    expect(currentBar1.value, closeTo(0.5, 0.01));
    expect(totalBar1.value, closeTo(0.25, 0.01));

    // Complete first upload; second upload starts
    firstUploadCompleter.complete(
      const SendToYotoResult(
        status: 'sent',
        cardId: 'card-bedtime',
        title: 'Bedtime Stories',
        chapterCount: 6,
        message: 'Appended',
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(
      find.textContaining('Current Upload: The Fellowship of the Ring'),
      findsOneWidget,
    );
    expect(find.text('Total Uploads (1 of 2)'), findsOneWidget);

    // Fail second upload -> should trigger error toast notification and hide banner once queue finishes
    secondUploadCompleter.completeError(
      const ApiException(502, '{"error":"Yoto API rate limit exceeded"}'),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byKey(const Key('yoto_upload_error_toast')), findsOneWidget);
    expect(
      find.textContaining('Yoto upload failed for "The Fellowship of the Ring"'),
      findsOneWidget,
    );
    expect(find.byKey(const Key('yoto_upload_queue_banner')), findsNothing);
  });
}

class _MockYotoRepository extends YotoRepository {
  String? lastSentBookId;
  String? lastSentSeriesId;
  SendToYotoRequest? lastRequest;
  final Map<String, Completer<SendToYotoResult>> bookUploadCompleters = {};
  YotoServerUploadProgress simulatedProgress =
      const YotoServerUploadProgress(isActive: false);

  _MockYotoRepository(SharedPreferences prefs)
      : super(
          apiService: ApiService(baseUrl: 'http://localhost:8080'),
          storageService: StorageService(prefs),
        );

  @override
  Future<YotoServerUploadProgress> getUploadProgress() async {
    return simulatedProgress;
  }

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

  @override
  Future<SendToYotoResult> sendBookToYoto(
    String bookId,
    SendToYotoRequest request,
  ) async {
    lastSentBookId = bookId;
    lastRequest = request;
    final completer = bookUploadCompleters[bookId];
    if (completer != null) {
      return completer.future;
    }
    return SendToYotoResult(
      status: 'sent',
      cardId: request.cardId ?? 'card-new',
      title: request.newPlaylistTitle ?? 'Bedtime Stories',
      chapterCount: 6,
      message: 'Appended to Yoto playlist',
    );
  }

  @override
  Future<SendToYotoResult> sendSeriesToYoto(
    String seriesId,
    SendToYotoRequest request,
  ) async {
    lastSentSeriesId = seriesId;
    lastRequest = request;
    return SendToYotoResult(
      status: 'sent',
      cardId: request.cardId ?? 'card-series-new',
      title: request.newPlaylistTitle ?? 'Middle-earth',
      chapterCount: 3,
      message: 'Sent series to Yoto playlist',
    );
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
