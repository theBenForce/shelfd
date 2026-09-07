import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/main.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('ShelfApp launches and renders connect screen initially', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: const ShelfApp(initialRoute: '/connect'),
      ),
    );

    expect(find.text('Connect to your Library'), findsOneWidget);
    expect(find.text('Scan Server QR Code'), findsOneWidget);
  });

  testWidgets('ShelfApp launches to library when initialRoute is /library', (tester) async {
    SharedPreferences.setMockInitialValues({'shelfd_auth_token': 'test-token'});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: const ShelfApp(initialRoute: '/library'),
      ),
    );

    expect(find.text('Shelfd'), findsOneWidget);
    expect(find.text('All Books'), findsOneWidget);
  });
}
