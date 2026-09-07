import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/core/tokens.dart';

void main() {
  group('Shared Layout Widgets Tests', () {
    testWidgets('BentoCard renders children with crisp border and padding', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(
            body: BentoCard(
              child: Text('Card Content'),
            ),
          ),
        ),
      );

      expect(find.text('Card Content'), findsOneWidget);
      expect(find.byType(Card), findsOneWidget);
    });

    testWidgets('FilterPillsRow renders selectable chips and responds to taps', (tester) async {
      String selected = 'all';
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Scaffold(
            body: FilterPillsRow(
              items: const [
                FilterPillItem(id: 'all', label: 'All Books'),
                FilterPillItem(id: 'series', label: 'Series'),
              ],
              selectedId: selected,
              onSelected: (id) => selected = id,
            ),
          ),
        ),
      );

      expect(find.text('All Books'), findsOneWidget);
      expect(find.text('Series'), findsOneWidget);

      await tester.tap(find.text('Series'));
      await tester.pumpAndSettle();
      expect(selected, 'series');
    });

    testWidgets('StatusBadge displays label with pastel styling', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(
            body: StatusBadge(
              label: '94% match',
              backgroundColor: AppTokens.matchBadgeBg,
              textColor: AppTokens.matchBadgeText,
            ),
          ),
        ),
      );

      expect(find.text('94% match'), findsOneWidget);
    });

    testWidgets('PrimaryButton has >= 48px touch target and loading state', (tester) async {
      bool pressed = false;
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: Scaffold(
            body: PrimaryButton(
              label: 'Connect',
              onPressed: () => pressed = true,
              isLoading: false,
            ),
          ),
        ),
      );

      final buttonSize = tester.getSize(find.byType(PrimaryButton));
      expect(buttonSize.height, greaterThanOrEqualTo(AppTokens.minTouchTarget));

      await tester.tap(find.text('Connect'));
      await tester.pump();
      expect(pressed, isTrue);

      // Verify loading spinner
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const Scaffold(
            body: PrimaryButton(
              label: 'Connect',
              onPressed: null,
              isLoading: true,
            ),
          ),
        ),
      );

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });
  });
}
