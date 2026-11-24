import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/data/models/cast_models.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/services/storage_service.dart';
import 'package:shelf/ui/features/audiobook/widgets/cast_device_sheet.dart';
import 'package:shelf/ui/state/audio_player_provider.dart';
import 'package:shelf/ui/state/cast_audio_player_engine.dart';
import 'package:shelf/ui/state/cast_provider.dart';
import 'package:shelf/ui/state/providers.dart';

class _FakeCastNotifier extends CastNotifier {
  final CastState _initial;
  CastDevice? lastConnected;
  bool disconnected = false;
  bool? lastCaptions;
  double? lastVolume;

  _FakeCastNotifier(this._initial);

  @override
  CastState build() => _initial;

  @override
  Future<void> refreshDevices({bool refresh = true}) async {}

  @override
  Future<void> connectToDevice(CastDevice device, {bool? captionsEnabled}) async {
    lastConnected = device;
  }

  @override
  Future<void> disconnectToLocal() async {
    disconnected = true;
  }

  @override
  Future<void> setCaptionsEnabled(bool enabled) async {
    lastCaptions = enabled;
  }

  @override
  Future<void> setCastVolume(double volume) async {
    lastVolume = volume;
  }
}

void main() {
  testWidgets('CastDeviceSheet renders devices, captions toggle for video TV, volume slider, and Stop Casting without overflow', (tester) async {
    tester.view.physicalSize = const Size(375, 812);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storage = StorageService(prefs);
    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'tok',
      client: MockClient((_) async => http.Response(jsonEncode({'devices': []}), 200)),
    );

    const tv = CastDevice(
      id: 'tv-1',
      name: 'Living Room TV With A Very Long Name For Ahem Overflow Test',
      model: 'Chromecast Ultra',
      host: '192.168.1.40',
      hasVideo: true,
    );
    const speaker = CastDevice(
      id: 'spk-1',
      name: 'Bedroom Speaker',
      model: 'Nest Audio',
      host: '192.168.1.41',
      hasVideo: false,
    );

    final fakeNotifier = _FakeCastNotifier(
      const CastState(
        devices: [tv, speaker],
        session: CastSessionSnapshot(
          sessionId: 'sess-1',
          userId: 'u1',
          device: tv,
          bookId: 'b1',
          phase: CastSessionPhase.playing,
          positionSeconds: 12,
          durationSeconds: 120,
          volume: 0.6,
          captionsEnabled: true,
        ),
      ),
    );

    final castEngine = CastAudioPlayerEngine(api);
    try {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            storageServiceProvider.overrideWithValue(storage),
            apiServiceProvider.overrideWithValue(api),
            audioPlayerEngineProvider.overrideWithValue(SimulatedAudioPlayerEngine()),
            castAudioEngineProvider.overrideWithValue(castEngine),
            castProvider.overrideWith(() => fakeNotifier),
          ],
          child: const MaterialApp(
            home: Scaffold(
              body: CastDeviceSheet(
                book: Book(id: 'b1', title: 'Peter Rabbit', hasReadalong: true),
                hasReadalong: true,
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('cast_sheet_this_device_tile')), findsOneWidget);
      expect(find.byKey(const Key('cast_device_tile_tv-1')), findsOneWidget);
      expect(find.byKey(const Key('cast_device_tile_spk-1')), findsOneWidget);
      expect(find.byKey(const Key('cast_sheet_captions_toggle')), findsOneWidget);
      expect(find.byKey(const Key('cast_sheet_volume_slider')), findsOneWidget);
      expect(find.byKey(const Key('cast_sheet_stop_button')), findsOneWidget);

      await tester.tap(find.byKey(const Key('cast_sheet_captions_toggle')));
      await tester.pumpAndSettle();
      expect(fakeNotifier.lastCaptions, isFalse);

      await tester.tap(find.byKey(const Key('cast_sheet_stop_button')));
      await tester.pumpAndSettle();
      expect(fakeNotifier.disconnected, isTrue);
    } finally {
      await castEngine.dispose();
    }
  });

  testWidgets('CastDeviceSheet hides TV captions toggle for audio-only speakers', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final storage = StorageService(prefs);
    final api = ApiService(
      baseUrl: 'http://localhost:8080',
      token: 'tok',
      client: MockClient((_) async => http.Response(jsonEncode({'devices': []}), 200)),
    );

    const speaker = CastDevice(
      id: 'spk-1',
      name: 'Bedroom Speaker',
      model: 'Nest Audio',
      host: '192.168.1.41',
      hasVideo: false,
    );

    final fakeNotifier = _FakeCastNotifier(
      const CastState(
        devices: [speaker],
        session: CastSessionSnapshot(
          sessionId: 'sess-2',
          userId: 'u1',
          device: speaker,
          bookId: 'b1',
          phase: CastSessionPhase.playing,
          positionSeconds: 5,
          durationSeconds: 120,
          volume: 0.5,
          captionsEnabled: false,
        ),
      ),
    );

    final castEngine = CastAudioPlayerEngine(api);
    try {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            storageServiceProvider.overrideWithValue(storage),
            apiServiceProvider.overrideWithValue(api),
            audioPlayerEngineProvider.overrideWithValue(SimulatedAudioPlayerEngine()),
            castAudioEngineProvider.overrideWithValue(castEngine),
            castProvider.overrideWith(() => fakeNotifier),
          ],
          child: const MaterialApp(
            home: Scaffold(
              body: CastDeviceSheet(
                book: Book(id: 'b1', title: 'Peter Rabbit', hasReadalong: true),
                hasReadalong: true,
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('cast_sheet_captions_toggle')), findsNothing);
      expect(find.byKey(const Key('cast_sheet_volume_slider')), findsOneWidget);
    } finally {
      await castEngine.dispose();
    }
  });
}
