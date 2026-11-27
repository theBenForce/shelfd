import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'wake_lock_stub.dart'
    if (dart.library.js_interop) 'wake_lock_web.dart'
    if (dart.library.io) 'wake_lock_io.dart' as impl;

abstract class ScreenWakeLock {
  Future<void> setEnabled(bool enabled);
}

ScreenWakeLock createPlatformScreenWakeLock() =>
    impl.createPlatformScreenWakeLock();

final screenWakeLockProvider = Provider<ScreenWakeLock>((ref) {
  final lock = createPlatformScreenWakeLock();
  ref.onDispose(() {
    lock.setEnabled(false);
  });
  return lock;
});
