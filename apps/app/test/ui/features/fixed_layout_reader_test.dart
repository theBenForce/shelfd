import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/book.dart';
import 'package:shelf/ui/features/reader/fixed_layout_reader.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late Book testFixedBook;
  late SharedPreferences prefs;

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    prefs = await SharedPreferences.getInstance();
    testFixedBook = const Book(
      id: 'coco-book-1',
      title: 'Coco Read-Along Storybook',
      layout: 'pre-paginated',
      spine: [
        SpineItem(
          id: 'p1',
          bookId: 'coco-book-1',
          chapterIndex: 1,
          title: 'Cover',
          pageWidth: 999,
          pageHeight: 999,
          pageSpread: 'right',
        ),
        SpineItem(
          id: 'p2',
          bookId: 'coco-book-1',
          chapterIndex: 2,
          title: 'Narrator & Cast',
          pageWidth: 999,
          pageHeight: 999,
          pageSpread: 'left',
        ),
        SpineItem(
          id: 'p3',
          bookId: 'coco-book-1',
          chapterIndex: 3,
          title: 'Title Page',
          pageWidth: 999,
          pageHeight: 999,
          pageSpread: 'right',
        ),
        SpineItem(
          id: 'p4',
          bookId: 'coco-book-1',
          chapterIndex: 4,
          title: 'Wide Gatefold Art',
          pageWidth: 2000,
          pageHeight: 1000,
          pageSpread: 'center',
        ),
        SpineItem(
          id: 'p5',
          bookId: 'coco-book-1',
          chapterIndex: 5,
          title: 'Page 5',
          pageWidth: 999,
          pageHeight: 999,
          pageSpread: 'left',
        ),
        SpineItem(
          id: 'p6',
          bookId: 'coco-book-1',
          chapterIndex: 6,
          title: 'Page 6',
          pageWidth: 999,
          pageHeight: 999,
          pageSpread: 'right',
        ),
      ],
    );
  });

  test('Book isFixedLayout evaluates true for pre-paginated books', () {
    expect(testFixedBook.isFixedLayout, isTrue);
    const reflowBook = Book(id: 'r1', title: 'Reflow Book', layout: 'reflowable');
    expect(reflowBook.isFixedLayout, isFalse);
  });

  testWidgets('FixedLayoutReader renders cover as single page on desktop', (tester) async {
    tester.view.physicalSize = const Size(1200, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          home: FixedLayoutReader(book: testFixedBook),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Coco Read-Along Storybook'), findsOneWidget);
    // On first spread (Cover), it shows Page 1 of 6
    expect(find.text('Page 1 of 6'), findsWidgets);
  });

  testWidgets('FixedLayoutReader navigates to facing spread (Pages 2-3) on Next click', (tester) async {
    tester.view.physicalSize = const Size(1200, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          home: FixedLayoutReader(book: testFixedBook),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Click Next Page button
    final nextBtn = find.byTooltip('Next Page');
    expect(nextBtn, findsOneWidget);
    await tester.tap(nextBtn);
    await tester.pumpAndSettle();

    // Facing two pages are paired: Pages 2–3
    expect(find.text('Pages 2–3 of 6'), findsOneWidget);
  });

  testWidgets('FixedLayoutReader in portrait mode renders single page', (tester) async {
    // Narrow portrait screen
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          home: FixedLayoutReader(book: testFixedBook),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Initial page: Page 1
    expect(find.text('Page 1 of 6'), findsWidgets);

    // Click Next Page button
    final nextBtn = find.byTooltip('Next Page');
    expect(nextBtn, findsOneWidget);
    await tester.tap(nextBtn);
    await tester.pumpAndSettle();

    // In portrait mode, it advances 1 page at a time (Page 2, not Pages 2-3)
    expect(find.text('Page 2 of 6'), findsWidgets);
  });

  testWidgets('FixedLayoutReader TOC drawer lists all pages and jumps to selection', (tester) async {
    tester.view.physicalSize = const Size(1200, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() => tester.view.resetPhysicalSize());

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: MaterialApp(
          home: FixedLayoutReader(book: testFixedBook),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Open TOC
    final tocBtn = find.byTooltip('Table of Contents');
    expect(tocBtn, findsOneWidget);
    await tester.tap(tocBtn);
    await tester.pumpAndSettle();

    expect(find.text('Pages & Chapters'), findsOneWidget);
    expect(find.text('Narrator & Cast'), findsOneWidget);
    expect(find.text('Wide Gatefold Art'), findsOneWidget);

    // Tap Wide Gatefold Art
    await tester.tap(find.text('Wide Gatefold Art'));
    await tester.pumpAndSettle();

    // Now on gatefold spread (Page 4)
    expect(find.text('Page 4 of 6'), findsWidgets);
  });
}
