import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/recap.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/recap/previously_on_sheet.dart';
import 'package:shelf/ui/state/providers.dart';

class _FakeRecapBookRepository extends BookRepository {
  final Future<BookRecap> Function({
    required String bookId,
    required int chapterIndex,
    required int paragraphOffset,
    required bool forceRefresh,
  }) onGetRecap;

  _FakeRecapBookRepository({
    required super.apiService,
    required super.storageService,
    required this.onGetRecap,
  });

  @override
  Future<BookRecap> getBookRecap(
    String bookId, {
    required int chapterIndex,
    int paragraphOffset = 0,
    bool forceRefresh = false,
  }) {
    return onGetRecap(
      bookId: bookId,
      chapterIndex: chapterIndex,
      paragraphOffset: paragraphOffset,
      forceRefresh: forceRefresh,
    );
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pumpSheet(
    WidgetTester tester, {
    required _FakeRecapBookRepository repo,
    int chapterIndex = 2,
  }) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          bookRepositoryProvider.overrideWithValue(repo),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Scaffold(
            body: PreviouslyOnSheet(
              bookId: 'book-dune',
              bookTitle: 'Dune',
              chapterIndex: chapterIndex,
            ),
          ),
        ),
      ),
    );
  }

  testWidgets('shows loading indicator then renders recap, ceiling badge, and cached badge',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final completer = Completer<BookRecap>();

    final repo = _FakeRecapBookRepository(
      apiService: ApiService(baseUrl: 'http://localhost:8080'),
      storageService: StorageService(prefs),
      onGetRecap: ({
        required bookId,
        required chapterIndex,
        required paragraphOffset,
        required forceRefresh,
      }) =>
          completer.future,
    );

    await pumpSheet(tester, repo: repo, chapterIndex: 2);
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.text('Building spoiler-free recap...'), findsOneWidget);
    expect(find.text('Up to Chapter 2'), findsOneWidget);

    completer.complete(
      const BookRecap(
        bookId: 'book-dune',
        bookTitle: 'Dune',
        chapterIndex: 2,
        recap: 'Paul endured the Gom Jabbar on Caladan and arrived on Arrakis.',
        cached: true,
        chaptersIncluded: 2,
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.text('Paul endured the Gom Jabbar on Caladan and arrived on Arrakis.'),
      findsOneWidget,
    );
    expect(find.text('Up to Chapter 2'), findsOneWidget);
    expect(find.byKey(const Key('previously_on_cached_badge')), findsOneWidget);
  });

  testWidgets('regenerate button sends forceRefresh=true and updates recap',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final forceRefreshCalls = <bool>[];

    final repo = _FakeRecapBookRepository(
      apiService: ApiService(baseUrl: 'http://localhost:8080'),
      storageService: StorageService(prefs),
      onGetRecap: ({
        required bookId,
        required chapterIndex,
        required paragraphOffset,
        required forceRefresh,
      }) async {
        forceRefreshCalls.add(forceRefresh);
        return BookRecap(
          bookId: bookId,
          bookTitle: 'Dune',
          chapterIndex: chapterIndex,
          recap: forceRefresh ? 'Fresh regenerated recap.' : 'Initial cached recap.',
          cached: !forceRefresh,
          chaptersIncluded: chapterIndex,
        );
      },
    );

    await pumpSheet(tester, repo: repo, chapterIndex: 3);
    await tester.pumpAndSettle();

    expect(find.text('Initial cached recap.'), findsOneWidget);
    expect(find.text('Up to Chapter 3'), findsOneWidget);
    expect(forceRefreshCalls, [false]);

    await tester.tap(find.byKey(const Key('previously_on_regenerate_button')));
    await tester.pumpAndSettle();

    expect(find.text('Fresh regenerated recap.'), findsOneWidget);
    expect(forceRefreshCalls, [false, true]);
    expect(find.byKey(const Key('previously_on_cached_badge')), findsNothing);
  });

  testWidgets('error state displays retry button and recovers cleanly',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    var attempts = 0;

    final repo = _FakeRecapBookRepository(
      apiService: ApiService(baseUrl: 'http://localhost:8080'),
      storageService: StorageService(prefs),
      onGetRecap: ({
        required bookId,
        required chapterIndex,
        required paragraphOffset,
        required forceRefresh,
      }) async {
        attempts++;
        if (attempts == 1) {
          throw Exception('AI backend offline');
        }
        return BookRecap(
          bookId: bookId,
          bookTitle: 'Dune',
          chapterIndex: chapterIndex,
          recap: 'Recovered spoiler-free recap.',
          cached: false,
          chaptersIncluded: chapterIndex,
        );
      },
    );

    await pumpSheet(tester, repo: repo, chapterIndex: 2);
    await tester.pumpAndSettle();

    expect(find.text('Could not generate recap'), findsOneWidget);
    expect(find.byKey(const Key('previously_on_retry_button')), findsOneWidget);

    await tester.tap(find.byKey(const Key('previously_on_retry_button')));
    await tester.pumpAndSettle();

    expect(find.text('Recovered spoiler-free recap.'), findsOneWidget);
    expect(attempts, 2);
  });
}
