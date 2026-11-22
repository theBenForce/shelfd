import 'readalong_pip_models.dart';

ReadalongPipBridge createReadalongPipBridge() => StubReadalongPipBridge();

class StubReadalongPipBridge implements ReadalongPipBridge {
  @override
  bool get isSupported => false;

  @override
  bool get isOpen => false;

  @override
  Future<bool> open(
    ReadalongPipSnapshot initial,
    ReadalongPipCallbacks callbacks,
  ) async =>
      false;

  @override
  void update(ReadalongPipSnapshot snapshot) {}

  @override
  void close() {}

  @override
  void dispose() {}
}
