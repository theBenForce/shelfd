import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/services/storage_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('StorageService Tests', () {
    setUp(() {
      SharedPreferences.setMockInitialValues({});
    });

    test('saves and retrieves server connection credentials', () async {
      final prefs = await SharedPreferences.getInstance();
      final storage = StorageService(prefs);

      expect(storage.getServerUrl(), isNull);
      expect(storage.getAuthToken(), isNull);

      await storage.saveServerConnection('http://192.168.1.50:8080', 'token-1234');
      expect(storage.getServerUrl(), 'http://192.168.1.50:8080');
      expect(storage.getAuthToken(), 'token-1234');

      await storage.clearServerConnection();
      expect(storage.getServerUrl(), isNull);
      expect(storage.getAuthToken(), isNull);
    });

    test('reading settings persistence defaults and updates', () async {
      final prefs = await SharedPreferences.getInstance();
      final storage = StorageService(prefs);

      expect(storage.getThemeMode(), 'bone');
      expect(storage.getFontSize(), 18.0);
      expect(storage.getLineHeight(), 1.6);
      expect(storage.getFontFamily(), 'serif');

      await storage.saveThemeMode('sepia');
      await storage.saveFontSize(22.0);
      await storage.saveLineHeight(1.8);
      await storage.saveFontFamily('sans');

      expect(storage.getThemeMode(), 'sepia');
      expect(storage.getFontSize(), 22.0);
      expect(storage.getLineHeight(), 1.8);
      expect(storage.getFontFamily(), 'sans');
    });

    test('chapter cache and book progress', () async {
      final prefs = await SharedPreferences.getInstance();
      final storage = StorageService(prefs);

      await storage.saveReadingProgress('book-1', 0.45);
      expect(storage.getReadingProgress('book-1'), 0.45);
      expect(storage.getReadingProgress('book-unknown'), 0.0);

      await storage.cacheChapter('book-1', 3, 'Chapter three content');
      expect(storage.getCachedChapter('book-1', 3), 'Chapter three content');
      expect(storage.getCachedChapter('book-1', 4), isNull);
    });
  });
}
