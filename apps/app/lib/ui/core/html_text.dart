import 'package:flutter/material.dart';
import 'tokens.dart';
import 'typography.dart';

enum HtmlBlockType {
  paragraph,
  h1,
  h2,
  h3,
  blockquote,
  listItem,
  divider,
}

class HtmlBlock {
  final HtmlBlockType type;
  final String htmlContent;
  final int listIndex;

  const HtmlBlock({
    required this.type,
    required this.htmlContent,
    this.listIndex = 0,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is HtmlBlock &&
          runtimeType == other.runtimeType &&
          type == other.type &&
          htmlContent == other.htmlContent &&
          listIndex == other.listIndex;

  @override
  int get hashCode => Object.hash(type, htmlContent, listIndex);

  @override
  String toString() => 'HtmlBlock(type: $type, htmlContent: "$htmlContent", listIndex: $listIndex)';
}

/// Decodes standard and numeric HTML entities.
String decodeHtmlEntities(String text) {
  if (!text.contains('&')) return text;

  var result = text
      .replaceAll('&amp;', '&')
      .replaceAll('&lt;', '<')
      .replaceAll('&gt;', '>')
      .replaceAll('&quot;', '"')
      .replaceAll('&apos;', "'")
      .replaceAll('&#39;', "'")
      .replaceAll('&nbsp;', ' ')
      .replaceAll('&mdash;', '—')
      .replaceAll('&ndash;', '–')
      .replaceAll('&hellip;', '…')
      .replaceAll('&ldquo;', '“')
      .replaceAll('&rdquo;', '”')
      .replaceAll('&lsquo;', '‘')
      .replaceAll('&rsquo;', '’')
      .replaceAll('&trade;', '™')
      .replaceAll('&copy;', '©')
      .replaceAll('&reg;', '®');

  result = result.replaceAllMapped(RegExp(r'&#(\d+);'), (match) {
    final code = int.tryParse(match.group(1)!);
    return code != null ? String.fromCharCode(code) : match.group(0)!;
  });

  result = result.replaceAllMapped(RegExp(r'&#x([0-9a-fA-F]+);', caseSensitive: false), (match) {
    final code = int.tryParse(match.group(1)!, radix: 16);
    return code != null ? String.fromCharCode(code) : match.group(0)!;
  });

  return result;
}

String _sanitizeHtml(String html) {
  return html
      .replaceAll(RegExp(r'<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>', caseSensitive: false), '')
      .replaceAll(RegExp(r'<style\b[^<]*(?:(?!<\/style>)<[^<]*)*<\/style>', caseSensitive: false), '');
}

final _blockTagRegex = RegExp(
  r'<(/?)(p|div|blockquote|h[1-6]|ul|ol|li|hr)\b([^>]*)>',
  caseSensitive: false,
);

/// Parses raw HTML into top-level structured blocks.
List<HtmlBlock> parseHtmlBlocks(String html) {
  final cleaned = _sanitizeHtml(html).trim();
  if (cleaned.isEmpty) return const [];

  if (!_blockTagRegex.hasMatch(cleaned)) {
    final normalized = cleaned.replaceAll(
      RegExp(r'(?:<br\s*/?>\s*){2,}', caseSensitive: false),
      '\n\n',
    );
    final paragraphs = normalized
        .split('\n\n')
        .map((p) => p.trim())
        .where((p) => p.isNotEmpty);

    return [
      for (final p in paragraphs)
        HtmlBlock(type: HtmlBlockType.paragraph, htmlContent: p),
    ];
  }

  final blocks = <HtmlBlock>[];
  var inOrderedList = false;
  var listIndex = 0;
  var currentType = HtmlBlockType.paragraph;
  final buffer = StringBuffer();
  var lastEnd = 0;

  void flushCurrentBlock() {
    final content = buffer.toString().trim();
    buffer.clear();
    if (content.isNotEmpty) {
      blocks.add(HtmlBlock(
        type: currentType,
        htmlContent: content,
        listIndex: currentType == HtmlBlockType.listItem && inOrderedList ? listIndex : 0,
      ));
    }
  }

  for (final match in _blockTagRegex.allMatches(cleaned)) {
    final prefix = cleaned.substring(lastEnd, match.start);
    buffer.write(prefix);

    final isClosing = match.group(1) == '/';
    final tag = match.group(2)!.toLowerCase();

    if (isClosing) {
      if (tag == 'ol') {
        flushCurrentBlock();
        inOrderedList = false;
        listIndex = 0;
      } else if (tag == 'ul') {
        flushCurrentBlock();
      } else {
        flushCurrentBlock();
        currentType = HtmlBlockType.paragraph;
      }
    } else {
      if (tag == 'hr') {
        flushCurrentBlock();
        blocks.add(const HtmlBlock(type: HtmlBlockType.divider, htmlContent: ''));
      } else if (tag == 'ol') {
        flushCurrentBlock();
        inOrderedList = true;
        listIndex = 0;
      } else if (tag == 'ul') {
        flushCurrentBlock();
        inOrderedList = false;
      } else if (tag == 'li') {
        flushCurrentBlock();
        currentType = HtmlBlockType.listItem;
        if (inOrderedList) listIndex++;
      } else {
        flushCurrentBlock();
        if (tag == 'h1') {
          currentType = HtmlBlockType.h1;
        } else if (tag == 'h2') {
          currentType = HtmlBlockType.h2;
        } else if (tag == 'h3' || tag == 'h4' || tag == 'h5' || tag == 'h6') {
          currentType = HtmlBlockType.h3;
        } else if (tag == 'blockquote') {
          currentType = HtmlBlockType.blockquote;
        } else {
          currentType = HtmlBlockType.paragraph;
        }
      }
    }

    lastEnd = match.end;
  }

  if (lastEnd < cleaned.length) {
    buffer.write(cleaned.substring(lastEnd));
  }
  flushCurrentBlock();

  return blocks;
}

class _StyleContext {
  final bool bold;
  final bool italic;
  final bool underline;
  final bool strikethrough;
  final bool code;
  final String? href;

  const _StyleContext({
    this.bold = false,
    this.italic = false,
    this.underline = false,
    this.strikethrough = false,
    this.code = false,
    this.href,
  });

  _StyleContext copyWith({
    bool? bold,
    bool? italic,
    bool? underline,
    bool? strikethrough,
    bool? code,
    String? href,
  }) {
    return _StyleContext(
      bold: bold ?? this.bold,
      italic: italic ?? this.italic,
      underline: underline ?? this.underline,
      strikethrough: strikethrough ?? this.strikethrough,
      code: code ?? this.code,
      href: href ?? this.href,
    );
  }
}

TextStyle _resolveStyle(_StyleContext ctx, TextStyle baseStyle, Color? linkColor) {
  var style = baseStyle;
  if (ctx.bold) {
    style = style.copyWith(fontWeight: FontWeight.bold);
  }
  if (ctx.italic) {
    style = style.copyWith(fontStyle: FontStyle.italic);
  }
  if (ctx.underline && ctx.strikethrough) {
    style = style.copyWith(
      decoration: TextDecoration.combine([
        TextDecoration.underline,
        TextDecoration.lineThrough,
      ]),
    );
  } else if (ctx.underline) {
    style = style.copyWith(decoration: TextDecoration.underline);
  } else if (ctx.strikethrough) {
    style = style.copyWith(decoration: TextDecoration.lineThrough);
  }
  if (ctx.code) {
    style = style.copyWith(fontFamily: 'monospace');
  }
  if (ctx.href != null) {
    style = style.copyWith(
      color: linkColor ?? Colors.blue,
      decoration: TextDecoration.underline,
    );
  }
  return style;
}

final _inlineTagRegex = RegExp(r'<(\/)?([a-zA-Z0-9]+)([^>]*)>');

/// Parses inline HTML markup into styled TextSpan elements.
List<InlineSpan> parseHtmlInlineSpans(
  String text,
  TextStyle baseStyle, {
  Color? linkColor,
}) {
  if (text.isEmpty) return const [];

  final spans = <InlineSpan>[];
  final stack = <_StyleContext>[const _StyleContext()];
  var lastIndex = 0;

  for (final match in _inlineTagRegex.allMatches(text)) {
    if (match.start > lastIndex) {
      var rawSegment = text.substring(lastIndex, match.start);
      if (spans.isNotEmpty && spans.last is TextSpan && (spans.last as TextSpan).text == '\n') {
        rawSegment = rawSegment.replaceFirst(RegExp(r'^\r?\n\s*'), '');
      }
      final decoded = decodeHtmlEntities(rawSegment);
      if (decoded.isNotEmpty) {
        spans.add(TextSpan(
          text: decoded,
          style: _resolveStyle(stack.last, baseStyle, linkColor),
        ));
      }
    }

    final isClosing = match.group(1) == '/';
    final tag = match.group(2)!.toLowerCase();

    if (tag == 'br') {
      spans.add(const TextSpan(text: '\n'));
    } else if (isClosing) {
      if (stack.length > 1) {
        stack.removeLast();
      }
    } else {
      String? href;
      if (tag == 'a') {
        final hrefMatch = RegExp(r'href=["\x27]([^"\x27]+)["\x27]', caseSensitive: false)
            .firstMatch(match.group(3) ?? '');
        href = hrefMatch?.group(1);
      }

      final current = stack.last;
      stack.add(current.copyWith(
        bold: (tag == 'b' || tag == 'strong') ? true : null,
        italic: (tag == 'i' || tag == 'em') ? true : null,
        underline: tag == 'u' ? true : null,
        strikethrough: (tag == 's' || tag == 'del' || tag == 'strike') ? true : null,
        code: tag == 'code' ? true : null,
        href: tag == 'a' ? href : null,
      ));
    }

    lastIndex = match.end;
  }

  if (lastIndex < text.length) {
    var rawSegment = text.substring(lastIndex);
    if (spans.isNotEmpty && spans.last is TextSpan && (spans.last as TextSpan).text == '\n') {
      rawSegment = rawSegment.replaceFirst(RegExp(r'^\r?\n\s*'), '');
    }
    final decoded = decodeHtmlEntities(rawSegment);
    if (decoded.isNotEmpty) {
      spans.add(TextSpan(
        text: decoded,
        style: _resolveStyle(stack.last, baseStyle, linkColor),
      ));
    }
  }

  return spans;
}

/// Renders HTML formatted text (e.g. book synopsis) with design system typography.
class HtmlText extends StatelessWidget {
  final String html;
  final TextStyle? style;
  final TextStyle? placeholderStyle;
  final String emptyPlaceholder;
  final double paragraphSpacing;

  const HtmlText({
    super.key,
    required this.html,
    this.style,
    this.placeholderStyle,
    this.emptyPlaceholder = 'No synopsis provided for this book.',
    this.paragraphSpacing = AppTokens.space12,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final effectiveStyle = style ?? AppTypography.bodySans(fontSize: 15, lineHeight: 1.6);
    final primaryColor = theme.colorScheme.primary;

    final trimmed = html.trim();
    if (trimmed.isEmpty) {
      return Text(
        emptyPlaceholder,
        style: placeholderStyle ?? effectiveStyle.copyWith(color: AppTokens.mutedCopy),
      );
    }

    final blocks = parseHtmlBlocks(trimmed);
    if (blocks.isEmpty) {
      return Text(
        emptyPlaceholder,
        style: placeholderStyle ?? effectiveStyle.copyWith(color: AppTokens.mutedCopy),
      );
    }

    if (blocks.length == 1) {
      return _buildBlockWidget(context, blocks.first, effectiveStyle, primaryColor);
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (int i = 0; i < blocks.length; i++) ...[
          if (i > 0) SizedBox(height: paragraphSpacing),
          _buildBlockWidget(context, blocks[i], effectiveStyle, primaryColor),
        ],
      ],
    );
  }

  Widget _buildBlockWidget(
    BuildContext context,
    HtmlBlock block,
    TextStyle baseStyle,
    Color primaryColor,
  ) {
    switch (block.type) {
      case HtmlBlockType.paragraph:
        return Text.rich(
          TextSpan(
            children: parseHtmlInlineSpans(
              block.htmlContent,
              baseStyle,
              linkColor: primaryColor,
            ),
          ),
        );

      case HtmlBlockType.h1:
        final h1Style = AppTypography.titleSerif(
          fontSize: 22.0,
          fontWeight: FontWeight.bold,
          color: primaryColor,
        );
        return Text.rich(
          TextSpan(
            children: parseHtmlInlineSpans(block.htmlContent, h1Style, linkColor: primaryColor),
          ),
        );

      case HtmlBlockType.h2:
        final h2Style = AppTypography.titleSerif(
          fontSize: 19.0,
          fontWeight: FontWeight.bold,
          color: primaryColor,
        );
        return Text.rich(
          TextSpan(
            children: parseHtmlInlineSpans(block.htmlContent, h2Style, linkColor: primaryColor),
          ),
        );

      case HtmlBlockType.h3:
        final h3Style = AppTypography.titleSerif(
          fontSize: 16.0,
          fontWeight: FontWeight.w600,
          color: primaryColor,
        );
        return Text.rich(
          TextSpan(
            children: parseHtmlInlineSpans(block.htmlContent, h3Style, linkColor: primaryColor),
          ),
        );

      case HtmlBlockType.blockquote:
        final quoteStyle = baseStyle.copyWith(
          fontStyle: FontStyle.italic,
          color: AppTokens.mutedCopy,
        );
        return Container(
          padding: const EdgeInsets.only(left: AppTokens.space12, top: 4, bottom: 4),
          decoration: BoxDecoration(
            border: Border(
              left: BorderSide(
                color: Theme.of(context).dividerColor,
                width: 3,
              ),
            ),
          ),
          child: Text.rich(
            TextSpan(
              children: parseHtmlInlineSpans(
                block.htmlContent,
                quoteStyle,
                linkColor: primaryColor,
              ),
            ),
          ),
        );

      case HtmlBlockType.listItem:
        final bulletText = block.listIndex > 0 ? '${block.listIndex}. ' : '• ';
        return Padding(
          padding: const EdgeInsets.only(left: AppTokens.space8),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                bulletText,
                style: baseStyle.copyWith(fontWeight: FontWeight.bold),
              ),
              Expanded(
                child: Text.rich(
                  TextSpan(
                    children: parseHtmlInlineSpans(
                      block.htmlContent,
                      baseStyle,
                      linkColor: primaryColor,
                    ),
                  ),
                ),
              ),
            ],
          ),
        );

      case HtmlBlockType.divider:
        return const Divider(height: 1, color: AppTokens.crispBorder);
    }
  }
}
