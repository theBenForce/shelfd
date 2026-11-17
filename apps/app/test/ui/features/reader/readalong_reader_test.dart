import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/author.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/ui/features/reader/fixed_layout_reader.dart';
import 'package:shelf/ui/state/providers.dart';

class _MockReadalongApiService extends ApiService {
  _MockReadalongApiService(this.manifest) : super(baseUrl: 'http://localhost:8080');

  final ReadalongManifest manifest;

  @override
  Future<ReadalongManifest> getBookReadalong(String id) async => manifest;

  @override
  Future<List<AudioChapter>> getAudioChapters(String bookId) async => const [];

  @override
  Future<AudiobookProgress> getAudiobookProgress(String bookId) async =>
      AudiobookProgress(bookId: bookId, positionSeconds: 0.0, speed: 1.0);

  @override
  Future<AudiobookProgress> saveAudiobookProgress(
    String bookId, {
    required double positionSeconds,
    double speed = 1.0,
    bool isCompleted = false,
  }) async =>
      AudiobookProgress(
        bookId: bookId,
        positionSeconds: positionSeconds,
        speed: speed,
        isCompleted: isCompleted,
      );

  @override
  String getAudioStreamUrl(String bookId) =>
      'http://localhost:8080/api/v1/audiobooks/$bookId/stream';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('ReadalongManifest & Book read-along model', () {
    test('parses has_readalong and SMIL segments accurately', () {
      final book = Book.fromJson({
        'id': 'book-1',
        'title': 'Four Silly Skeletons',
        'book_type': 'ebook',
        'has_readalong': true,
        'duration_seconds': 9.0,
        'authors': [
          {'id': 'a1', 'name': 'Mark Sperring & Sue Hendra'},
        ],
      });

      expect(book.isAudiobook, isFalse);
      expect(book.hasReadalong, isTrue);
      expect(book.canReadAlong, isTrue);
      expect(book.durationSeconds, 9.0);

      final manifest = ReadalongManifest.fromJson({
        'book_id': 'book-1',
        'has_readalong': true,
        'has_media_overlays': true,
        'active_class': '-epub-media-overlay-active',
        'playback_active_class': '-epub-media-overlay-playing',
        'duration_seconds': 9.0,
        'segments': [
          {
            'chapter_index': 1,
            'fragment_id': 'w1',
            'text': 'Four silly skeletons',
            'clip_begin': 0.0,
            'clip_end': 4.0,
            'global_begin': 0.0,
            'global_end': 4.0,
          },
          {
            'chapter_index': 2,
            'fragment_id': 'w2',
            'text': 'dancing in the night!',
            'clip_begin': 0.0,
            'clip_end': 5.0,
            'global_begin': 4.0,
            'global_end': 9.0,
          },
        ],
      });

      expect(manifest.segmentAt(1.5)?.fragmentId, 'w1');
      expect(manifest.segmentAt(1.5)?.text, 'Four silly skeletons');
      expect(manifest.segmentAt(5.2)?.fragmentId, 'w2');
      expect(manifest.segmentAt(5.2)?.chapterIndex, 2);
    });
  });

  group('FixedLayoutReader Read-Along playback, highlighting, and page turning', () {
    testWidgets('plays audio, highlights spoken phrases, and turns pages automatically',
        (tester) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();

      final manifest = ReadalongManifest(
        bookId: 'b-skeletons',
        hasReadalong: true,
        hasMediaOverlays: true,
        durationSeconds: 9.0,
        segments: const [
          ReadalongSegment(
            chapterIndex: 1,
            fragmentId: 'w1',
            text: 'Four silly skeletons',
            clipBegin: 0.0,
            clipEnd: 4.0,
            globalBegin: 0.0,
            globalEnd: 4.0,
          ),
          ReadalongSegment(
            chapterIndex: 2,
            fragmentId: 'w2',
            text: 'dancing in the night!',
            clipBegin: 0.0,
            clipEnd: 5.0,
            globalBegin: 4.0,
            globalEnd: 9.0,
          ),
        ],
      );

      final book = Book(
        id: 'b-skeletons',
        title: 'Four Silly Skeletons',
        layout: 'pre-paginated',
        bookType: 'ebook',
        hasReadalong: true,
        durationSeconds: 9.0,
        authors: const [Author(id: 'a1', name: 'Mark Sperring & Sue Hendra')],
        spine: const [
          SpineItem(
            id: 'ch1',
            bookId: 'b-skeletons',
            chapterIndex: 1,
            title: 'Page 1',
            pageWidth: 600,
            pageHeight: 800,
          ),
          SpineItem(
            id: 'ch2',
            bookId: 'b-skeletons',
            chapterIndex: 2,
            title: 'Page 2',
            pageWidth: 600,
            pageHeight: 800,
          ),
        ],
      );

      final engine = SimulatedAudioPlayerEngine(
        initialDuration: const Duration(seconds: 9),
      );
      final api = _MockReadalongApiService(manifest);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            sharedPreferencesProvider.overrideWithValue(prefs),
            apiServiceProvider.overrideWithValue(api),
            audioPlayerEngineProvider.overrideWithValue(engine),
          ],
          child: MaterialApp(
            home: FixedLayoutReader(
              book: book,
              initialReadalongManifest: manifest,
            ),
          ),
        ),
      );
      await tester.pump();

      // Starts on Page 1 of 2
      expect(find.text('Page 1 of 2'), findsOneWidget);
      expect(find.byKey(const Key('readalong-play-pause-button')), findsOneWidget);

      // Tap Play Read-Along
      await tester.tap(find.byKey(const Key('readalong-play-pause-button')));
      await tester.pump();

      // Seek to 1.0s -> Chapter 1 phrase "Four silly skeletons" is highlighted
      await engine.seek(const Duration(seconds: 1));
      await tester.pump();
      expect(find.text('Four silly skeletons'), findsOneWidget);
      expect(find.text('Page 1 of 2'), findsOneWidget);

      // Seek to 5.0s -> Advances to Chapter 2 phrase "dancing in the night!" and auto-turns to Page 2
      await engine.seek(const Duration(seconds: 5));
      await tester.pump();
      expect(find.text('dancing in the night!'), findsOneWidget);
      expect(find.text('Page 2 of 2'), findsOneWidget);

      await engine.stop();
      await tester.pump();
    });
  });
}
