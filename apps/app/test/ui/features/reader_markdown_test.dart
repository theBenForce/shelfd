import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/reader/reader_markdown.dart';
import 'package:shelf/ui/state/providers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('parseReaderBlocks', () {
    test('parses headings correctly', () {
      final markdown = '# Main Heading\n\n## Subheading\n\n### Section\n\n#### Subsection';
      final blocks = parseReaderBlocks(markdown);

      expect(blocks.length, 4);
      expect(blocks[0].type, ReaderBlockType.h1);
      expect(blocks[0].text, 'Main Heading');

      expect(blocks[1].type, ReaderBlockType.h2);
      expect(blocks[1].text, 'Subheading');

      expect(blocks[2].type, ReaderBlockType.h3);
      expect(blocks[2].text, 'Section');

      expect(blocks[3].type, ReaderBlockType.h4);
      expect(blocks[3].text, 'Subsection');
    });

    test('parses blockquotes and dividers', () {
      final markdown = '> A quote from history.\n> Second quote line.\n\n---\n\nNormal paragraph.';
      final blocks = parseReaderBlocks(markdown);

      expect(blocks.length, 3);
      expect(blocks[0].type, ReaderBlockType.blockquote);
      expect(blocks[0].text, 'A quote from history.\nSecond quote line.');

      expect(blocks[1].type, ReaderBlockType.divider);

      expect(blocks[2].type, ReaderBlockType.paragraph);
      expect(blocks[2].text, 'Normal paragraph.');
    });
  });

  group('parseInlineSpans', () {
    test('parses inline bold, italic, bold-italic, and footnotes', () {
      const text = 'Here is a **bold** word, an *italic* thought, ***both*** together, and a footnote[5].';
      const baseStyle = TextStyle(fontSize: 16, color: Colors.black);
      const accent = Colors.red;

      final spans = parseInlineSpans(text, baseStyle, accent);

      // Verify non-empty spans produced
      expect(spans.isNotEmpty, true);

      // Verify footnote span is a WidgetSpan
      final widgetSpans = spans.whereType<WidgetSpan>().toList();
      expect(widgetSpans.length, 1);

      // Verify text spans
      final textSpans = spans.whereType<TextSpan>().toList();
      expect(textSpans.any((s) => s.text == 'bold' && s.style?.fontWeight == FontWeight.bold), true);
      expect(textSpans.any((s) => s.text == 'italic' && s.style?.fontStyle == FontStyle.italic), true);
      expect(
        textSpans.any(
          (s) => s.text == 'both' && s.style?.fontWeight == FontWeight.bold && s.style?.fontStyle == FontStyle.italic,
        ),
        true,
      );
    });
  });

  group('buildReaderBlockWidget', () {
    testWidgets('renders H2 and paragraph with footnote widget', (tester) async {
      const h2Block = ReaderBlock(type: ReaderBlockType.h2, text: 'The reign of the Dixiecrats');
      const pBlock = ReaderBlock(
        type: ReaderBlockType.paragraph,
        text: 'Red-blooded men know what I mean.”[5]',
      );

      final settings = const ReaderSettings();
      final theme = AppTheme.buildTheme(ReadingThemeMode.bone);

      await tester.pumpWidget(
        MaterialApp(
          theme: theme,
          home: Scaffold(
            body: Column(
              children: [
                buildReaderBlockWidget(block: h2Block, settings: settings, theme: theme),
                buildReaderBlockWidget(block: pBlock, settings: settings, theme: theme),
              ],
            ),
          ),
        ),
      );

      expect(find.text('The reign of the Dixiecrats'), findsOneWidget);
      expect(find.textContaining('Red-blooded men know what I mean.”'), findsOneWidget);
      expect(find.text('5'), findsOneWidget);
    });
  });
}
