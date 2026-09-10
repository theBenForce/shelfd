import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/connect/connect_view.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('ConnectView displays QR option, manual inputs, and privacy badge',
      (tester) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const ConnectView(),
        ),
      ),
    );

    // Verify header elements
    expect(find.text('Connect to your Library'), findsOneWidget);
    expect(find.text('Scan Server QR Code'), findsOneWidget);
    expect(find.text('Or connect manually'), findsOneWidget);

    // Verify input fields
    expect(find.widgetWithText(TextField, 'Server URL'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'Username'), findsOneWidget);
    expect(find.widgetWithText(TextField, 'Password'), findsOneWidget);

    // Verify connect action button
    expect(find.text('Connect & Sync Library'), findsOneWidget);

    // Verify privacy footer
    expect(find.textContaining('Direct connection to your private server'), findsOneWidget);
  });

  test('resolveDefaultServerUrl returns fallback localhost on non-web platforms', () {
    expect(resolveDefaultServerUrl(), 'http://localhost:8080');
  });

  testWidgets('ConnectView pre-fills with saved server URL from preferences', (tester) async {
    SharedPreferences.setMockInitialValues({
      'shelfd_server_url': 'https://shelfd.thebenforce.com',
      'shelfd_saved_username': 'customuser',
    });
    final prefs = await SharedPreferences.getInstance();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const ConnectView(),
        ),
      ),
    );

    expect(find.text('https://shelfd.thebenforce.com'), findsOneWidget);
    expect(find.text('customuser'), findsOneWidget);
  });
}
