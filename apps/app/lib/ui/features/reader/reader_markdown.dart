import 'package:flutter/material.dart';
import 'package:shelf/ui/core/tokens.dart';
import 'package:shelf/ui/core/typography.dart';
import 'package:shelf/ui/state/providers.dart';

enum ReaderBlockType {
  h1,
  h2,
  h3,
  h4,
  blockquote,
  divider,
  paragraph,
}

class ReaderBlock {
  final ReaderBlockType type;
  final String text;

  const ReaderBlock({
    required this.type,
    required this.text,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ReaderBlock &&
          runtimeType == other.runtimeType &&
          type == other.type &&
          text == other.text;

  @override
  int get hashCode => type.hashCode ^ text.hashCode;

  @override
  String toString() => 'ReaderBlock(type: $type, text: "$text")';
}

/// Parses raw chapter content into structured reading blocks.
List<ReaderBlock> parseReaderBlocks(String rawContent) {
  if (rawContent.trim().isEmpty) return const [];

  final rawParagraphs = rawContent
      .split('\n\n')
      .map((p) => p.trim())
      .where((p) => p.isNotEmpty);

  final blocks = <ReaderBlock>[];

  for (final p in rawParagraphs) {
    if (p == '---' || p == '***' || p == '* * *' || p == '___') {
      blocks.add(const ReaderBlock(type: ReaderBlockType.divider, text: ''));
    } else if (p.startsWith('# ')) {
      blocks.add(ReaderBlock(type: ReaderBlockType.h1, text: p.substring(2).trim()));
    } else if (p.startsWith('## ')) {
      blocks.add(ReaderBlock(type: ReaderBlockType.h2, text: p.substring(3).trim()));
    } else if (p.startsWith('### ')) {
      blocks.add(ReaderBlock(type: ReaderBlockType.h3, text: p.substring(4).trim()));
    } else if (p.startsWith('#### ')) {
      blocks.add(ReaderBlock(type: ReaderBlockType.h4, text: p.substring(5).trim()));
    } else if (p.startsWith('> ')) {
      // Clean leading '>' from lines
      final cleanLines = p
          .split('\n')
          .map((line) => line.trim().startsWith('> ') ? line.trim().substring(2).trim() : line.trim())
          .join('\n');
      blocks.add(ReaderBlock(type: ReaderBlockType.blockquote, text: cleanLines));
    } else {
      blocks.add(ReaderBlock(type: ReaderBlockType.paragraph, text: p));
    }
  }

  return blocks;
}

final _inlineTokenRegex = RegExp(
  r'(\[\d+\])|(\*\*\*[^*]+\*\*\*)|(\*\*[^*]+\*\*)|(\*[^*]+\*)',
);

/// Parses markdown inline tokens (bold, italic, footnote brackets) into rich text spans.
List<InlineSpan> parseInlineSpans(
  String text,
  TextStyle baseStyle,
  Color footnoteColor,
) {
  final spans = <InlineSpan>[];
  int lastIndex = 0;

  for (final match in _inlineTokenRegex.allMatches(text)) {
    if (match.start > lastIndex) {
      spans.add(TextSpan(
        text: text.substring(lastIndex, match.start),
        style: baseStyle,
      ));
    }

    final matchText = match.group(0)!;

    if (match.group(1) != null) {
      // Footnote reference, e.g. "[4]" -> Render as superscript
      final numText = matchText.substring(1, matchText.length - 1);
      spans.add(
        WidgetSpan(
          alignment: PlaceholderAlignment.middle,
          child: Transform.translate(
            offset: const Offset(1, -4),
            child: Text(
              numText,
              style: TextStyle(
                fontFamily: baseStyle.fontFamily,
                fontFamilyFallback: baseStyle.fontFamilyFallback,
                fontSize: (baseStyle.fontSize ?? 16) * 0.72,
                fontWeight: FontWeight.w700,
                color: footnoteColor,
              ),
            ),
          ),
        ),
      );
    } else if (match.group(2) != null) {
      // Bold Italic: ***text***
      final inner = matchText.substring(3, matchText.length - 3);
      spans.add(TextSpan(
        text: inner,
        style: baseStyle.copyWith(
          fontWeight: FontWeight.bold,
          fontStyle: FontStyle.italic,
        ),
      ));
    } else if (match.group(3) != null) {
      // Bold: **text**
      final inner = matchText.substring(2, matchText.length - 2);
      spans.add(TextSpan(
        text: inner,
        style: baseStyle.copyWith(
          fontWeight: FontWeight.bold,
        ),
      ));
    } else if (match.group(4) != null) {
      // Italic: *text*
      final inner = matchText.substring(1, matchText.length - 1);
      spans.add(TextSpan(
        text: inner,
        style: baseStyle.copyWith(
          fontStyle: FontStyle.italic,
        ),
      ));
    }

    lastIndex = match.end;
  }

  if (lastIndex < text.length) {
    spans.add(TextSpan(
      text: text.substring(lastIndex),
      style: baseStyle,
    ));
  }

  return spans;
}

/// Renders a single ReaderBlock widget matching the user's typography settings and theme.
Widget buildReaderBlockWidget({
  required ReaderBlock block,
  required ReaderSettings settings,
  required ThemeData theme,
}) {
  final primaryColor = theme.colorScheme.primary;
  final onSurfaceColor = theme.colorScheme.onSurface;

  switch (block.type) {
    case ReaderBlockType.h1:
      final style = AppTypography.titleSerif(
        fontSize: (settings.fontSize * 1.5).clamp(24.0, 34.0),
        fontWeight: FontWeight.bold,
        color: primaryColor,
      );
      return Padding(
        padding: const EdgeInsets.only(top: 28, bottom: 14),
        child: Text.rich(
          TextSpan(children: parseInlineSpans(block.text, style, primaryColor)),
        ),
      );

    case ReaderBlockType.h2:
      final style = AppTypography.titleSerif(
        fontSize: (settings.fontSize * 1.3).clamp(20.0, 28.0),
        fontWeight: FontWeight.w700,
        color: primaryColor,
      );
      return Padding(
        padding: const EdgeInsets.only(top: 24, bottom: 12),
        child: Text.rich(
          TextSpan(children: parseInlineSpans(block.text, style, primaryColor)),
        ),
      );

    case ReaderBlockType.h3:
      final style = AppTypography.titleSerif(
        fontSize: (settings.fontSize * 1.15).clamp(18.0, 24.0),
        fontWeight: FontWeight.w600,
        color: primaryColor,
      );
      return Padding(
        padding: const EdgeInsets.only(top: 20, bottom: 10),
        child: Text.rich(
          TextSpan(children: parseInlineSpans(block.text, style, primaryColor)),
        ),
      );

    case ReaderBlockType.h4:
      final style = AppTypography.titleSerif(
        fontSize: settings.fontSize,
        fontWeight: FontWeight.w600,
        color: primaryColor,
      );
      return Padding(
        padding: const EdgeInsets.only(top: 16, bottom: 8),
        child: Text.rich(
          TextSpan(children: parseInlineSpans(block.text, style, primaryColor)),
        ),
      );

    case ReaderBlockType.divider:
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: AppTokens.space24),
        child: Center(
          child: SizedBox(
            width: 80,
            child: Divider(
              color: theme.dividerColor,
              thickness: 1.5,
            ),
          ),
        ),
      );

    case ReaderBlockType.blockquote:
      final quoteStyle = AppTypography.readerText(
        fontSize: settings.fontSize * 0.95,
        lineHeight: settings.lineHeight,
        isSerif: settings.isSerif,
        color: onSurfaceColor.withValues(alpha: 0.85),
      ).copyWith(fontStyle: FontStyle.italic);

      return Container(
        margin: const EdgeInsets.only(bottom: AppTokens.space16),
        padding: const EdgeInsets.only(left: AppTokens.space16, top: 4, bottom: 4),
        decoration: BoxDecoration(
          border: Border(
            left: BorderSide(
              color: primaryColor.withValues(alpha: 0.45),
              width: 3,
            ),
          ),
        ),
        child: Text.rich(
          TextSpan(children: parseInlineSpans(block.text, quoteStyle, primaryColor)),
        ),
      );

    case ReaderBlockType.paragraph:
      final bodyStyle = AppTypography.readerText(
        fontSize: settings.fontSize,
        lineHeight: settings.lineHeight,
        isSerif: settings.isSerif,
        color: onSurfaceColor,
      );
      return Padding(
        padding: const EdgeInsets.only(bottom: AppTokens.space16),
        child: Text.rich(
          TextSpan(children: parseInlineSpans(block.text, bodyStyle, primaryColor)),
        ),
      );
  }
}
