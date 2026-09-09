import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/ui/core/html_text.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('decodeHtmlEntities', () {
    test('decodes common named HTML entities', () {
      const input = 'Tom &amp; Jerry &quot;Show&quot; &apos;50s&#39; &lt;hit&gt; &mdash; &ndash; &hellip; &ldquo;great&rdquo;';
      final decoded = decodeHtmlEntities(input);
      expect(decoded, 'Tom & Jerry "Show" \'50s\' <hit> — – … “great”');
    });

    test('decodes decimal and hexadecimal numeric entities', () {
      const input = '&#169; 2026 &#x26; &#x3c;safe&#x3e;';
      final decoded = decodeHtmlEntities(input);
      expect(decoded, '© 2026 & <safe>');
    });

    test('returns original string when no ampersand present', () {
      const input = 'Plain text without entities';
      expect(decodeHtmlEntities(input), input);
    });
  });

  group('parseHtmlBlocks', () {
    test('parses plain text into paragraph blocks', () {
      const text = 'First paragraph.\n\nSecond paragraph.';
      final blocks = parseHtmlBlocks(text);

      expect(blocks.length, 2);
      expect(blocks[0].type, HtmlBlockType.paragraph);
      expect(blocks[0].htmlContent, 'First paragraph.');
      expect(blocks[1].type, HtmlBlockType.paragraph);
      expect(blocks[1].htmlContent, 'Second paragraph.');
    });

    test('treats double br tags as paragraph breaks when no block tags present', () {
      const text = 'First part.<br><br>Second part.<br /><br />Third part.';
      final blocks = parseHtmlBlocks(text);

      expect(blocks.length, 3);
      expect(blocks[0].htmlContent, 'First part.');
      expect(blocks[1].htmlContent, 'Second part.');
      expect(blocks[2].htmlContent, 'Third part.');
    });

    test('parses p tags into individual paragraph blocks', () {
      const html = '<p>Paragraph 1</p><p>Paragraph 2</p>';
      final blocks = parseHtmlBlocks(html);

      expect(blocks.length, 2);
      expect(blocks[0].type, HtmlBlockType.paragraph);
      expect(blocks[0].htmlContent, 'Paragraph 1');
      expect(blocks[1].type, HtmlBlockType.paragraph);
      expect(blocks[1].htmlContent, 'Paragraph 2');
    });

    test('parses headings, blockquotes, lists, and dividers', () {
      const html = '''
        <h1>Title</h1>
        <h2>Subtitle</h2>
        <blockquote>A wise quote.</blockquote>
        <hr />
        <ul>
          <li>Bullet item 1</li>
          <li>Bullet item 2</li>
        </ul>
        <ol>
          <li>Numbered item 1</li>
          <li>Numbered item 2</li>
        </ol>
      ''';
      final blocks = parseHtmlBlocks(html);

      expect(blocks.any((b) => b.type == HtmlBlockType.h1), true);
      expect(blocks.any((b) => b.type == HtmlBlockType.h2), true);
      expect(blocks.any((b) => b.type == HtmlBlockType.blockquote), true);
      expect(blocks.any((b) => b.type == HtmlBlockType.divider), true);

      final listItems = blocks.where((b) => b.type == HtmlBlockType.listItem).toList();
      expect(listItems.length, 4);
      expect(listItems[0].listIndex, 0); // unordered
      expect(listItems[1].listIndex, 0); // unordered
      expect(listItems[2].listIndex, 1); // ordered 1
      expect(listItems[3].listIndex, 2); // ordered 2
    });

    test('strips script and style elements', () {
      const html = '<p>Safe text</p><script>alert("xss")</script><style>.bad{}</style>';
      final blocks = parseHtmlBlocks(html);

      expect(blocks.length, 1);
      expect(blocks[0].htmlContent, 'Safe text');
      expect(blocks[0].htmlContent.contains('script'), false);
      expect(blocks[0].htmlContent.contains('bad'), false);
    });
  });

  group('parseHtmlInlineSpans', () {
    const baseStyle = TextStyle(fontSize: 15, color: Colors.black);

    test('parses bold, italic, and underline tags', () {
      const text = 'Normal <b>bold text</b> and <i>italic text</i> and <u>underlined</u>.';
      final spans = parseHtmlInlineSpans(text, baseStyle);

      expect(spans.isNotEmpty, true);

      final textSpans = spans.whereType<TextSpan>().toList();
      final boldSpan = textSpans.firstWhere((s) => s.text == 'bold text');
      expect(boldSpan.style?.fontWeight, FontWeight.bold);

      final italicSpan = textSpans.firstWhere((s) => s.text == 'italic text');
      expect(italicSpan.style?.fontStyle, FontStyle.italic);

      final underlineSpan = textSpans.firstWhere((s) => s.text == 'underlined');
      expect(underlineSpan.style?.decoration, TextDecoration.underline);
    });

    test('parses nested bold and italic tags', () {
      const text = '<b>Bold and <i>italic</i></b>';
      final spans = parseHtmlInlineSpans(text, baseStyle);

      final textSpans = spans.whereType<TextSpan>().toList();
      final nestedSpan = textSpans.firstWhere((s) => s.text == 'italic');
      expect(nestedSpan.style?.fontWeight, FontWeight.bold);
      expect(nestedSpan.style?.fontStyle, FontStyle.italic);
    });

    test('translates br tags to newline spans', () {
      const text = 'Line 1<br>Line 2<br/>Line 3';
      final spans = parseHtmlInlineSpans(text, baseStyle);

      final textSpans = spans.whereType<TextSpan>().toList();
      final newlineCount = textSpans.where((s) => s.text == '\n').length;
      expect(newlineCount, 2);
    });

    test('handles unrecognized tags without showing raw markup', () {
      const text = '<span class="author"><mark>Highlighted</mark></span>';
      final spans = parseHtmlInlineSpans(text, baseStyle);

      final combined = spans.whereType<TextSpan>().map((s) => s.text).join();
      expect(combined, 'Highlighted');
      expect(combined.contains('<span'), false);
      expect(combined.contains('<mark'), false);
    });
  });

  group('HtmlText Widget', () {
    testWidgets('renders placeholder when html is empty', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: HtmlText(
              html: '',
              emptyPlaceholder: 'No synopsis available.',
            ),
          ),
        ),
      );

      expect(find.text('No synopsis available.'), findsOneWidget);
    });

    testWidgets('renders plain text synopsis cleanly', (tester) async {
      const plainSynopsis = 'A groundbreaking work of science fiction set on the planet Gethen.';
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: HtmlText(html: plainSynopsis),
          ),
        ),
      );

      expect(find.text(plainSynopsis), findsOneWidget);
    });

    testWidgets('renders complex EPUB synopsis HTML without exposing raw tags', (tester) async {
      const htmlSynopsis =
          '<p><b>From bar bets to Nobel Prizes, viral outbreaks to lottery wins.</b><br>'
          'We live in an uncertain world. In <i>What Are the Odds?</i> Mark Prell reveals...<br>'
          'Through unforgettable stories of statistical ingenuity.</p>';

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: HtmlText(html: htmlSynopsis),
          ),
        ),
      );

      // Verify no raw tags appear anywhere in the widget tree text
      expect(find.textContaining('<p>'), findsNothing);
      expect(find.textContaining('<b>'), findsNothing);
      expect(find.textContaining('</b>'), findsNothing);
      expect(find.textContaining('<br>'), findsNothing);
      expect(find.textContaining('<i>'), findsNothing);
      expect(find.textContaining('</i>'), findsNothing);
      expect(find.textContaining('</p>'), findsNothing);

      // Verify the formatted text chunks are rendered
      expect(find.textContaining('From bar bets to Nobel Prizes'), findsOneWidget);
      expect(find.textContaining('What Are the Odds?'), findsOneWidget);
      expect(find.textContaining('Through unforgettable stories'), findsOneWidget);

      // Verify bold style was applied to the hook
      final richTextWidget = tester.widget<RichText>(find.byType(RichText));
      final rootSpan = richTextWidget.text as TextSpan;
      final flatSpans = <TextSpan>[];
      void collectSpans(InlineSpan s) {
        if (s is TextSpan) {
          flatSpans.add(s);
          s.children?.forEach(collectSpans);
        }
      }
      collectSpans(rootSpan);

      final boldSpan = flatSpans.firstWhere(
        (s) => s.text?.contains('From bar bets') ?? false,
      );
      expect(boldSpan.style?.fontWeight, FontWeight.bold);

      final italicSpan = flatSpans.firstWhere(
        (s) => s.text?.contains('What Are the Odds?') ?? false,
      );
      expect(italicSpan.style?.fontStyle, FontStyle.italic);
    });
  });
}
