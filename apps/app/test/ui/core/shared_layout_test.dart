import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/models/queue_status.dart';
import 'package:shelf/ui/core/shared_layout.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/core/tokens.dart';
import 'package:shelf/ui/state/providers.dart';

class FakeQueueNotifier extends QueueNotifier {
  final QueueState _initial;
  FakeQueueNotifier([this._initial = const QueueState()]);

  @override
  QueueState build() => _initial;
}

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

    testWidgets('ShelfdSideNav renders branding, items, and connection status', (tester) async {
      int selectedNav = 0;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            queueProvider.overrideWith(() => FakeQueueNotifier()),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: ShelfdSideNav(
                currentIndex: selectedNav,
                onTap: (i) => selectedNav = i,
              ),
            ),
          ),
        ),
      );

      expect(find.text('Shelfd'), findsOneWidget);
      expect(find.text('Your Digital Vellum'), findsOneWidget);
      expect(find.text('Library'), findsOneWidget);
      expect(find.text('Semantic Search'), findsOneWidget);
      expect(find.text('Settings'), findsOneWidget);
      expect(find.text('Homelab NAS - Connected'), findsOneWidget);

      await tester.tap(find.text('Semantic Search'));
      await tester.pump();
      expect(selectedNav, 1);
    });

    testWidgets('ShelfdSideNav renders active AI Indexing progress card', (tester) async {
      final activeStatus = const QueueStatus(
        totalChapters: 45,
        indexedChapters: 15,
        pendingChapters: 30,
        pendingUploads: 0,
        progressPercent: 33.3,
        isActive: true,
        currentBook: 'Dune Messiah',
        currentChapter: 'Chapter 4',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            queueProvider.overrideWith(() => FakeQueueNotifier(QueueState(status: activeStatus))),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: ShelfdSideNav(
                currentIndex: 0,
                onTap: (_) {},
              ),
            ),
          ),
        ),
      );

      expect(find.text('AI Indexing'), findsOneWidget);
      expect(find.text('33%'), findsOneWidget);
      expect(find.text('Dune Messiah'), findsOneWidget);
      expect(find.text('Chapter 4'), findsOneWidget);
    });

    testWidgets('ShelfdSideNav renders synced badge when idle', (tester) async {
      final idleStatus = const QueueStatus(
        totalChapters: 50,
        indexedChapters: 50,
        pendingChapters: 0,
        pendingUploads: 0,
        progressPercent: 100.0,
        isActive: false,
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            queueProvider.overrideWith(() => FakeQueueNotifier(QueueState(status: idleStatus))),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: Scaffold(
              body: ShelfdSideNav(
                currentIndex: 0,
                onTap: (_) {},
              ),
            ),
          ),
        ),
      );

      expect(find.text('AI Catalog Synced (50 ch)'), findsOneWidget);
    });

    testWidgets('ShelfdAdaptiveScaffold switches between ShelfdBottomNav and ShelfdSideNav', (tester) async {
      tester.view.physicalSize = const Size(600, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            queueProvider.overrideWith(() => FakeQueueNotifier()),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: ShelfdAdaptiveScaffold(
              currentIndex: 0,
              onNavTap: (_) {},
              body: const Center(child: Text('Content')),
            ),
          ),
        ),
      );

      expect(find.byType(ShelfdBottomNav), findsOneWidget);
      expect(find.byType(ShelfdSideNav), findsNothing);

      tester.view.physicalSize = const Size(1440, 900);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            queueProvider.overrideWith(() => FakeQueueNotifier()),
          ],
          child: MaterialApp(
            theme: AppTheme.buildTheme(ReadingThemeMode.bone),
            home: ShelfdAdaptiveScaffold(
              currentIndex: 0,
              onNavTap: (_) {},
              body: const Center(child: Text('Content')),
            ),
          ),
        ),
      );

      expect(find.byType(ShelfdSideNav), findsOneWidget);
      expect(find.byType(ShelfdBottomNav), findsNothing);
    });
  });
}
