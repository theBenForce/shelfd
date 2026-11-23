import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/cast_models.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/state/audio_player_provider.dart';
import 'package:shelf/ui/state/audio_player_state.dart';
import 'package:shelf/ui/state/cast_audio_player_engine.dart';
import 'package:shelf/ui/state/cast_provider.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  group('CastModels', () {
    test('parses CastDevice and CastSessionSnapshot from server JSON', () {
      final snap = CastSessionSnapshot.fromJson({
        'session_id': 'sess-1',
        'user_id': 'user-1',
        'device': {
          'id': 'tv-1',
          'name': 'Living Room TV',
          'model': 'Chromecast Ultra',
          'host': '192.168.1.40',
          'port': 8009,
          'has_video': true,
          'has_audio': true,
        },
        'book_id': 'book-1',
        'phase': 'playing',
        'position_seconds': 14.5,
        'duration_seconds': 120.0,
        'speed': 1.5,
        'volume': 0.7,
        'muted': false,
        'captions_enabled': true,
      });

      expect(snap.isActive, isTrue);
      expect(snap.device.name, 'Living Room TV');
      expect(snap.device.hasVideo, isTrue);
      expect(snap.toAudioPlaybackStatus(), AudioPlaybackStatus.playing);
      expect(snap.positionSeconds, 14.5);
      expect(snap.speed, 1.5);
      expect(snap.captionsEnabled, isTrue);
    });
  });

  group('CastAudioPlayerEngine', () {
    test('interpolates position smoothly at 1.5x and drives ReadalongManifest segments', () async {
      final recordedActions = <String>[];
      final mockClient = MockClient((req) async {
        if (req.url.path == '/api/v1/cast/session/control') {
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          recordedActions.add(body['action'].toString());
          return http.Response(
            jsonEncode({
              'session': {
                'session_id': 'sess-1',
                'user_id': 'u1',
                'device': {'id': 'tv-1', 'name': 'Living Room TV', 'host': '127.0.0.1'},
                'book_id': 'b1',
                'phase': body['action'] == 'pause' ? 'paused' : 'playing',
                'position_seconds': body['position_seconds'] ?? 10.0,
                'duration_seconds': 60.0,
                'speed': body['speed'] ?? 1.5,
              },
            }),
            200,
          );
        }
        return http.Response('{}', 200);
      });

      final api = ApiService(baseUrl: 'http://localhost:8080', token: 'tok', client: mockClient);
      final engine = CastAudioPlayerEngine(
        api,
        tickInterval: const Duration(milliseconds: 50),
      );
      addTearDown(engine.dispose);

      final manifest = ReadalongManifest(
        bookId: 'b1',
        hasReadalong: true,
        hasMediaOverlays: true,
        activeClass: 'active',
        playbackActiveClass: 'playing',
        segments: const [
          ReadalongSegment(
            chapterIndex: 0,
            chapterHref: 'page01.xhtml',
            fragmentId: 'w1',
            text: 'Once',
            clipBegin: 0,
            clipEnd: 0.2,
            globalBegin: 0,
            globalEnd: 0.2,
          ),
          ReadalongSegment(
            chapterIndex: 0,
            chapterHref: 'page01.xhtml',
            fragmentId: 'w2',
            text: 'upon',
            clipBegin: 0.2,
            clipEnd: 1.0,
            globalBegin: 0.2,
            globalEnd: 1.0,
          ),
        ],
      );

      final seenPositions = <Duration>[];
      final sub = engine.positionStream.listen(seenPositions.add);
      addTearDown(sub.cancel);

      engine.applyServerSnapshot(
        const CastSessionSnapshot(
          sessionId: 'sess-1',
          userId: 'u1',
          device: CastDevice(id: 'tv-1', name: 'Living Room TV', host: '127.0.0.1'),
          bookId: 'b1',
          phase: CastSessionPhase.playing,
          positionSeconds: 0.0,
          durationSeconds: 60.0,
          speed: 1.5,
        ),
        forceSnap: true,
      );

      await Future<void>.delayed(const Duration(milliseconds: 220));
      expect(seenPositions.length, greaterThanOrEqualTo(3));
      expect(seenPositions.last.inMilliseconds, greaterThanOrEqualTo(200));

      final seg = manifest.segmentAt(seenPositions.last.inMilliseconds / 1000.0);
      expect(seg?.fragmentId, 'w2');

      await engine.seek(const Duration(seconds: 25));
      expect(recordedActions, contains('seek'));
      expect(engine.currentInterpolatedPosition().inSeconds, greaterThanOrEqualTo(25));

      await engine.pause();
      expect(recordedActions, contains('pause'));
    });

    test('handoffToCast preserves readalong mode and suppresses redundant HTTP progress POSTs', () async {
      SharedPreferences.setMockInitialValues({
        'shelfd_server_url': 'http://localhost:8080',
        'shelfd_auth_token': 'tok',
      });
      final prefs = await SharedPreferences.getInstance();
      final storage = StorageService(prefs);

      int progressPosts = 0;
      final mockClient = MockClient((req) async {
        if (req.url.path.endsWith('/progress') && req.method == 'POST') {
          progressPosts++;
          return http.Response('{}', 200);
        }
        return http.Response('{}', 200);
      });
      final api = ApiService(baseUrl: 'http://localhost:8080', token: 'tok', client: mockClient);
      final localEngine = SimulatedAudioPlayerEngine();
      final castEngine = CastAudioPlayerEngine(api);

      final container = ProviderContainer(
        overrides: [
          storageServiceProvider.overrideWithValue(storage),
          apiServiceProvider.overrideWithValue(api),
          audioPlayerEngineProvider.overrideWithValue(localEngine),
          castAudioEngineProvider.overrideWithValue(castEngine),
        ],
      );
      addTearDown(container.dispose);

      final notifier = container.read(audioPlayerProvider.notifier);
      const book = Book(
        id: 'book-ra-1',
        title: 'The Tale of Peter Rabbit',
        hasReadalong: true,
        durationSeconds: 9.0,
      );
      await notifier.loadBook(book, initialPosition: 2.0, autoPlay: false);
      notifier.setPlaybackMode(AudioPlaybackMode.readalong);

      const device = CastDevice(
        id: 'tv-1',
        name: 'Living Room TV',
        host: '192.168.1.40',
        hasVideo: true,
      );
      await notifier.handoffToCast(
        device,
        snapshot: const CastSessionSnapshot(
          sessionId: 'sess-1',
          userId: 'u1',
          device: device,
          bookId: 'book-ra-1',
          phase: CastSessionPhase.playing,
          positionSeconds: 2.5,
          durationSeconds: 9.0,
          speed: 1.25,
          captionsEnabled: true,
        ),
      );

      final castState = container.read(audioPlayerProvider);
      expect(castState.isCasting, isTrue);
      expect(castState.activeCastDevice?.name, 'Living Room TV');
      expect(castState.playbackMode, AudioPlaybackMode.readalong);
      expect(castState.speed, 1.25);

      final postsBeforePause = progressPosts;
      await notifier.pause();
      expect(progressPosts, postsBeforePause);

      await notifier.handoffToLocal();
      final localState = container.read(audioPlayerProvider);
      expect(localState.isCasting, isFalse);
      expect(localState.playbackMode, AudioPlaybackMode.readalong);
      expect(localState.position.inMilliseconds, greaterThanOrEqualTo(2500));
    });
  });
}
