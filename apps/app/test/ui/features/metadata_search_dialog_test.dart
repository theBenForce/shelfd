import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/metadata_search_result.dart';
import 'package:shelf/data/repositories/book_repository.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/book_detail/metadata_search_dialog.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late StorageService storageService;

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    storageService = StorageService(prefs);
  });

  group('MetadataSearchDialog Tests', () {
    testWidgets('renders search dialog and allows searching and selecting results', (tester) async {
      final mockResults = [
        {
          'provider': 'openlibrary',
          'title': 'Foundation',
          'author': 'Isaac Asimov',
          'series': 'Foundation',
          'series_sequence': '1',
          'description': 'The story of our future.',
          'cover_url': null,
          'publisher': 'Gnome Press',
          'published_year': 1951,
          'isbn': '9780553293357',
          'genres': ['Science Fiction'],
        },
      ];

      final mockClient = MockClient((request) async {
        if (request.url.path == '/api/v1/metadata/search') {
          return http.Response(jsonEncode(mockResults), 200, headers: {
            'content-type': 'application/json',
          });
        }
        return http.Response('{}', 200);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      MetadataSearchResult? selectedResult;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(bookRepo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: Builder(
                builder: (context) => ElevatedButton(
                  onPressed: () async {
                    selectedResult = await showMetadataSearchDialog(
                      context,
                      initialTitle: 'Foundation',
                      initialAuthor: 'Isaac Asimov',
                    );
                  },
                  child: const Text('Open Search'),
                ),
              ),
            ),
          ),
        ),
      );

      // Open the dialog
      await tester.tap(find.text('Open Search'));
      await tester.pumpAndSettle();

      expect(find.text('Search Metadata'), findsOneWidget);
      expect(find.text('Foundation'), findsWidgets);
      expect(find.text('Isaac Asimov'), findsOneWidget);
      expect(find.text('Open Library'), findsOneWidget);
      expect(find.text('Apply'), findsOneWidget);

      // Tap Apply to select the result
      await tester.tap(find.text('Apply'));
      await tester.pumpAndSettle();

      expect(selectedResult, isNotNull);
      expect(selectedResult!.title, 'Foundation');
      expect(selectedResult!.author, 'Isaac Asimov');
      expect(selectedResult!.provider, 'openlibrary');
    });

    testWidgets('renders search dialog in ReadingThemeMode.dark with Apple and Google badges',
        (tester) async {
      final mockResults = [
        {
          'provider': 'googlebooks',
          'title': 'Dune',
          'author': 'Frank Herbert',
          'series': 'Dune Chronicles',
          'series_sequence': '1',
          'description': 'Arrakis.',
          'cover_url': null,
          'publisher': 'Chilton Books',
          'published_year': 1965,
          'isbn': '9780441172719',
          'genres': ['Science Fiction'],
        },
        {
          'provider': 'apple',
          'title': 'Dune Messiah',
          'author': 'Frank Herbert',
          'series': 'Dune Chronicles',
          'series_sequence': '2',
          'description': 'Sequel.',
          'cover_url': null,
          'publisher': 'Putnam',
          'published_year': 1969,
          'isbn': '9780441172696',
          'genres': ['Science Fiction'],
        },
      ];

      final mockClient = MockClient((request) async {
        if (request.url.path == '/api/v1/metadata/search') {
          return http.Response(jsonEncode(mockResults), 200, headers: {
            'content-type': 'application/json',
          });
        }
        return http.Response('{}', 200);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(bookRepo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.dark),
            home: Scaffold(
              body: Builder(
                builder: (context) => ElevatedButton(
                  onPressed: () {
                    showMetadataSearchDialog(
                      context,
                      initialTitle: 'Dune',
                      initialAuthor: 'Frank Herbert',
                    );
                  },
                  child: const Text('Open Search Dark'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open Search Dark'));
      await tester.pumpAndSettle();

      expect(find.text('Google Books'), findsOneWidget);
      expect(find.text('Apple Books'), findsOneWidget);
      expect(find.text('Series: Dune Chronicles #1'), findsOneWidget);
      expect(find.text('• ISBN: 9780441172719'), findsOneWidget);
    });

    testWidgets('MetadataSearchDialogContent defaults isBottomSheet to false and renders No metadata matches found',
        (tester) async {
      final mockClient = MockClient((request) async {
        if (request.url.path == '/api/v1/metadata/search') {
          return http.Response(jsonEncode([]), 200, headers: {
            'content-type': 'application/json',
          });
        }
        return http.Response('{}', 200);
      });

      final apiService = ApiService(baseUrl: 'http://localhost:8080', client: mockClient);
      final bookRepo = BookRepository(apiService: apiService, storageService: storageService);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            bookRepositoryProvider.overrideWithValue(bookRepo),
            storageServiceProvider.overrideWithValue(storageService),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: const Scaffold(
              body: MetadataSearchDialogContent(
                initialTitle: 'Obscure Manuscript',
              ),
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.textContaining('No metadata matches found'), findsOneWidget);
    });
  });
}
