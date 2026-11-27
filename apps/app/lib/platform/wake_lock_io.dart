import 'dart:io';
import 'package:flutter/services.dart';
import 'wake_lock.dart';

class _NativeScreenWakeLock implements ScreenWakeLock {
  static const MethodChannel _channel =
      MethodChannel('com.justwriteapps.shelfd/wakelock');

  bool? _lastEnabled;

  @override
  Future<void> setEnabled(bool enabled) async {
    if (_lastEnabled == enabled) return;
    _lastEnabled = enabled;
    if (Platform.environment.containsKey('FLUTTER_TEST')) {
      return;
    }
    if (!Platform.isAndroid && !Platform.isIOS) {
      return;
    }
    try {
      await _channel.invokeMethod<bool>(
        'setKeepScreenOn',
        <String, Object?>{'enabled': enabled},
      );
    } catch (_) {}
  }
}

ScreenWakeLock createPlatformScreenWakeLock() => _NativeScreenWakeLock();
