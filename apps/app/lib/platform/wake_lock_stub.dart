import 'wake_lock.dart';

class _StubScreenWakeLock implements ScreenWakeLock {
  @override
  Future<void> setEnabled(bool enabled) async {}
}

ScreenWakeLock createPlatformScreenWakeLock() => _StubScreenWakeLock();
