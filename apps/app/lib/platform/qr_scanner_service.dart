import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

class QrScannerUnavailableException implements Exception {
  final String message;

  const QrScannerUnavailableException([
    this.message = 'Google Code Scanner is unavailable on this device.',
  ]);

  @override
  String toString() => message;
}

class QrScannerService {
  static const String channelName = 'com.justwriteapps.shelfd/qr_scanner';

  final MethodChannel _channel;
  final StreamController<String> _pairUriController =
      StreamController<String>.broadcast();
  bool _handlerBound = false;

  QrScannerService({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel(channelName) {
    _ensureHandlerBound();
  }

  void _ensureHandlerBound() {
    if (_handlerBound || kIsWeb) return;
    _handlerBound = true;
    try {
      _channel.setMethodCallHandler((MethodCall call) async {
        if (call.method == 'onPairUri') {
          final uri = call.arguments?.toString().trim();
          if (uri != null && uri.isNotEmpty && !_pairUriController.isClosed) {
            _pairUriController.add(uri);
          }
        }
      });
    } catch (_) {}
  }

  Stream<String> get pairUriStream {
    _ensureHandlerBound();
    return _pairUriController.stream;
  }

  Future<String?> scanQrCode() async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) {
      throw const QrScannerUnavailableException(
        'Native QR code scanning is only available on Android.',
      );
    }
    try {
      final raw = await _channel.invokeMethod<String>('scanQrCode');
      final trimmed = raw?.trim();
      if (trimmed == null || trimmed.isEmpty) {
        return null;
      }
      return trimmed;
    } on PlatformException catch (e) {
      if (e.code == 'CANCELED') {
        return null;
      }
      throw QrScannerUnavailableException(
        e.message ?? 'QR scanner unavailable (${e.code}).',
      );
    } on MissingPluginException {
      throw const QrScannerUnavailableException();
    }
  }

  Future<String> getDeviceModel() async {
    if (kIsWeb) {
      return 'Web Browser';
    }
    if (defaultTargetPlatform == TargetPlatform.android) {
      try {
        final raw = await _channel.invokeMethod<String>('getDeviceModel');
        final trimmed = raw?.trim();
        if (trimmed != null && trimmed.isNotEmpty) {
          return trimmed;
        }
      } catch (_) {}
      return 'Android';
    }
    switch (defaultTargetPlatform) {
      case TargetPlatform.iOS:
        return 'iOS';
      case TargetPlatform.macOS:
        return 'macOS';
      case TargetPlatform.linux:
        return 'Linux';
      case TargetPlatform.windows:
        return 'Windows';
      default:
        return 'Device';
    }
  }

  Future<String> getDeviceTokenLabel() async {
    final model = await getDeviceModel();
    if (defaultTargetPlatform == TargetPlatform.android) {
      return 'Shelfd Android ($model)';
    }
    return 'Shelfd ($model)';
  }

  Future<String?> getInitialPairUri() async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) {
      return null;
    }
    try {
      final raw = await _channel.invokeMethod<String>('getInitialPairUri');
      final trimmed = raw?.trim();
      if (trimmed == null || trimmed.isEmpty) {
        return null;
      }
      return trimmed;
    } catch (_) {
      return null;
    }
  }

  Future<bool> setWebViewCookie({
    required String url,
    required String cookie,
  }) async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) {
      return false;
    }
    try {
      final ok = await _channel.invokeMethod<bool>(
        'setWebViewCookie',
        <String, Object?>{
          'url': url,
          'cookie': cookie,
        },
      );
      return ok ?? false;
    } catch (_) {
      return false;
    }
  }

  Future<void> clearWebViewCookies() async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) {
      return;
    }
    try {
      await _channel.invokeMethod<bool>('clearWebViewCookies');
    } catch (_) {}
  }

  void emitPairUriForTest(String uri) {
    if (!_pairUriController.isClosed) {
      _pairUriController.add(uri);
    }
  }

  void dispose() {
    _pairUriController.close();
  }
}
