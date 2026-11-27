import 'dart:js_interop';
import 'dart:js_interop_unsafe';
import 'package:web/web.dart' as web;
import 'wake_lock.dart';

class _WebScreenWakeLock implements ScreenWakeLock {
  bool _desiredEnabled = false;
  JSObject? _sentinel;
  bool _visibilityListenerAttached = false;

  void _ensureVisibilityListener() {
    if (_visibilityListenerAttached) return;
    _visibilityListenerAttached = true;
    web.document.addEventListener(
      'visibilitychange',
      (web.Event _) {
        if (_desiredEnabled && web.document.visibilityState == 'visible') {
          _acquire();
        }
      }.toJS,
    );
  }

  Future<void> _acquire() async {
    if (!_desiredEnabled || _sentinel != null) return;
    try {
      final nav = web.window.navigator as JSObject;
      if (!nav.has('wakeLock')) return;
      final wakeLock = nav.getProperty<JSAny?>('wakeLock'.toJS);
      if (wakeLock == null || !wakeLock.isA<JSObject>()) return;
      final promise =
          (wakeLock as JSObject).callMethod<JSPromise<JSAny?>>('request'.toJS, 'screen'.toJS);
      final sentinel = await promise.toDart;
      if (!_desiredEnabled) {
        if (sentinel != null && sentinel.isA<JSObject>()) {
          try {
            (sentinel as JSObject).callMethod<JSAny?>('release'.toJS);
          } catch (_) {}
        }
        return;
      }
      if (sentinel != null && sentinel.isA<JSObject>()) {
        _sentinel = sentinel as JSObject;
      }
    } catch (_) {}
  }

  Future<void> _release() async {
    final sentinel = _sentinel;
    _sentinel = null;
    if (sentinel == null) return;
    try {
      final promise = sentinel.callMethod<JSAny?>('release'.toJS);
      if (promise != null && promise.isA<JSPromise<JSAny?>>()) {
        await (promise as JSPromise<JSAny?>).toDart;
      }
    } catch (_) {}
  }

  @override
  Future<void> setEnabled(bool enabled) async {
    if (_desiredEnabled == enabled) return;
    _desiredEnabled = enabled;
    if (enabled) {
      _ensureVisibilityListener();
      await _acquire();
    } else {
      await _release();
    }
  }
}

ScreenWakeLock createPlatformScreenWakeLock() => _WebScreenWakeLock();
