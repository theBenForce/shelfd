import 'package:flutter/material.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/core/tokens.dart';
import 'package:shelf/ui/core/typography.dart';
import 'package:shelf/ui/state/providers.dart';

import 'dart:math' as math;
import 'package:flutter/gestures.dart';
import 'package:shelf/data/models/highlight.dart';

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
  final int paragraphIndex;
  final int startOffset;
  final int endOffset;

  const ReaderBlock({
    required this.type,
    required this.text,
    this.paragraphIndex = 1,
    this.startOffset = 0,
    this.endOffset = 0,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ReaderBlock &&
          runtimeType == other.runtimeType &&
          type == other.type &&
          text == other.text &&
          paragraphIndex == other.paragraphIndex &&
          startOffset == other.startOffset &&
          endOffset == other.endOffset;

  @override
  int get hashCode =>
      type.hashCode ^
      text.hashCode ^
      paragraphIndex.hashCode ^
      startOffset.hashCode ^
      endOffset.hashCode;

  @override
  String toString() =>
      'ReaderBlock(type: $type, para: $paragraphIndex, offset: $startOffset-$endOffset, text: "$text")';
}

/// Parses raw chapter content into structured reading blocks.
List<ReaderBlock> parseReaderBlocks(String rawContent) {
  if (rawContent.trim().isEmpty) return const [];

  final normalized = rawContent.replaceAll('\r\n', '\n');
  final rawParagraphs = normalized.split('\n\n');

  final blocks = <ReaderBlock>[];
  int currentOffset = 0;
  int paraIndex = 1;

  for (final rawP in rawParagraphs) {
    final trimmed = rawP.trim();
    if (trimmed.isEmpty) {
      currentOffset += rawP.length + 2;
      continue;
    }

    final blockStart = normalized.indexOf(rawP, currentOffset);
    final actualStart = blockStart != -1 ? blockStart : currentOffset;
    final actualEnd = actualStart + rawP.length;
    currentOffset = actualEnd + 2;

    ReaderBlockType type;
    String text;

    if (trimmed == '---' || trimmed == '***' || trimmed == '* * *' || trimmed == '___') {
      type = ReaderBlockType.divider;
      text = '';
    } else if (trimmed.startsWith('# ')) {
      type = ReaderBlockType.h1;
      text = trimmed.substring(2).trim();
    } else if (trimmed.startsWith('## ')) {
      type = ReaderBlockType.h2;
      text = trimmed.substring(3).trim();
    } else if (trimmed.startsWith('### ')) {
      type = ReaderBlockType.h3;
      text = trimmed.substring(4).trim();
    } else if (trimmed.startsWith('#### ')) {
      type = ReaderBlockType.h4;
      text = trimmed.substring(5).trim();
    } else if (trimmed.startsWith('> ')) {
      type = ReaderBlockType.blockquote;
      text = trimmed
          .split('\n')
          .map((line) => line.trim().startsWith('> ') ? line.trim().substring(2).trim() : line.trim())
          .join('\n');
    } else {
      type = ReaderBlockType.paragraph;
      text = trimmed;
    }

    blocks.add(ReaderBlock(
      type: type,
      text: text,
      paragraphIndex: paraIndex++,
      startOffset: actualStart,
      endOffset: actualEnd,
    ));
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
              style: AppTypography.mono(
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

class HighlightRange {
  final int startOffset;
  final int endOffset;
  final int startParagraph;
  final int endParagraph;

  int get start => startOffset;
  int get end => endOffset;

  const HighlightRange({
    required this.startOffset,
    required this.endOffset,
    required this.startParagraph,
    required this.endParagraph,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is HighlightRange &&
          runtimeType == other.runtimeType &&
          startOffset == other.startOffset &&
          endOffset == other.endOffset &&
          startParagraph == other.startParagraph &&
          endParagraph == other.endParagraph;

  @override
  int get hashCode => Object.hash(startOffset, endOffset, startParagraph, endParagraph);

  @override
  String toString() =>
      'HighlightRange(startOffset: $startOffset, endOffset: $endOffset, startParagraph: $startParagraph, endParagraph: $endParagraph)';
}

class _FindHighlightRangeCallable {
  const _FindHighlightRangeCallable();

  HighlightRange? call({
    List<ReaderBlock> blocks = const [],
    String fullContent = '',
    String selectedText = '',
  }) {
    final effectiveBlocks = blocks.isNotEmpty ? blocks : parseReaderBlocks(fullContent);
    return _findHighlightRangeImpl(
      blocks: effectiveBlocks,
      fullContent: fullContent,
      selectedText: selectedText,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) {
    if (invocation.isMethod && invocation.memberName == #call) {
      final pos = invocation.positionalArguments;
      if (pos.length >= 2) {
        final fullContent = pos[0]?.toString() ?? '';
        final second = pos[1];
        String selectedText = '';
        int? explicitStart;
        int? explicitEnd;
        if (second is Highlight) {
          selectedText = second.selectedText;
          explicitStart = second.startOffset;
          explicitEnd = second.endOffset;
        } else if (second is String) {
          selectedText = second;
        }
        final blocks = parseReaderBlocks(fullContent);
        final computed = _findHighlightRangeImpl(
          blocks: blocks,
          fullContent: fullContent,
          selectedText: selectedText,
        );
        if (computed != null) return computed;
        if (explicitStart != null &&
            explicitEnd != null &&
            explicitStart >= 0 &&
            explicitEnd > explicitStart &&
            explicitEnd <= fullContent.length) {
          return HighlightRange(
            startOffset: explicitStart,
            endOffset: explicitEnd,
            startParagraph: 1,
            endParagraph: 1,
          );
        }
        return null;
      }
    }
    return super.noSuchMethod(invocation);
  }
}

/// Computes the global offset range and paragraph bounds for selected text across reader blocks.
/// Supports both named parameters `(blocks:, fullContent:, selectedText:)` and positional `(String, Highlight|String)`.
const dynamic findHighlightRange = _FindHighlightRangeCallable();

HighlightRange? _findHighlightRangeImpl({
  required List<ReaderBlock> blocks,
  required String fullContent,
  required String selectedText,
}) {
  final trimmed = selectedText.trim();
  if (trimmed.isEmpty) return null;

  // 1. Exact match within a single block
  for (final block in blocks) {
    final idx = block.text.indexOf(trimmed);
    if (idx != -1) {
      final start = block.startOffset + idx;
      return HighlightRange(
        startOffset: start,
        endOffset: start + trimmed.length,
        startParagraph: block.paragraphIndex,
        endParagraph: block.paragraphIndex,
      );
    }
  }

  // 2. Multi-paragraph selection: match first and last non-empty segment
  final parts = trimmed
      .split(RegExp(r'\n+'))
      .map((s) => s.trim())
      .where((s) => s.isNotEmpty)
      .toList();

  if (parts.length > 1) {
    final firstPart = parts.first;
    final lastPart = parts.last;

    int? startOffset;
    int? startPara;
    int? endOffset;
    int? endPara;

    for (final block in blocks) {
      if (startOffset == null) {
        final idx = block.text.indexOf(firstPart);
        if (idx != -1) {
          startOffset = block.startOffset + idx;
          startPara = block.paragraphIndex;
        }
      }

      if (startOffset != null) {
        final idx = block.text.lastIndexOf(lastPart);
        if (idx != -1) {
          endOffset = block.startOffset + idx + lastPart.length;
          endPara = block.paragraphIndex;
        }
      }
    }

    if (startOffset != null && endOffset != null && endOffset > startOffset) {
      return HighlightRange(
        startOffset: startOffset,
        endOffset: endOffset,
        startParagraph: startPara ?? 0,
        endParagraph: endPara ?? (startPara ?? 0),
      );
    }
  }

  // 3. Fallback: Direct search in fullContent
  final idx = fullContent.indexOf(trimmed);
  if (idx != -1) {
    final end = idx + trimmed.length;
    int startPara = 0;
    int endPara = 0;
    for (final block in blocks) {
      if (idx >= block.startOffset && idx <= block.endOffset) {
        startPara = block.paragraphIndex;
      }
      if (end >= block.startOffset && end <= block.endOffset) {
        endPara = block.paragraphIndex;
      }
    }
    return HighlightRange(
      startOffset: idx,
      endOffset: end,
      startParagraph: startPara,
      endParagraph: endPara >= startPara ? endPara : startPara,
    );
  }

  // 4. Normalized whitespace fallback in fullContent
  final tokens = trimmed.split(RegExp(r'\s+')).where((t) => t.isNotEmpty).map(RegExp.escape).toList();
  if (tokens.isNotEmpty) {
    final pattern = RegExp(tokens.join(r'\s+'));
    final match = pattern.firstMatch(fullContent);
    if (match != null) {
      return HighlightRange(
        startOffset: match.start,
        endOffset: match.end,
        startParagraph: 1,
        endParagraph: 1,
      );
    }
  }

  return null;
}

class _HighlightSegment {
  final int start;
  final int end;
  final Highlight highlight;

  const _HighlightSegment({
    required this.start,
    required this.end,
    required this.highlight,
  });
}

/// Builds rich text spans for a block, factoring in active highlights and inline markdown tokens.
List<InlineSpan> buildBlockSpans({
  required ReaderBlock block,
  required TextStyle baseStyle,
  required Color accentColor,
  required ReaderSettings settings,
  List<Highlight> highlights = const [],
  void Function(Highlight hl)? onHighlightTap,
}) {
  if (highlights.isEmpty || block.text.isEmpty) {
    return parseInlineSpans(block.text, baseStyle, accentColor);
  }

  final segments = <_HighlightSegment>[];

  for (final hl in highlights) {
    int? segStart;
    int? segEnd;

    // 1. Offset-based overlap calculation
    if (hl.startOffset != null && hl.endOffset != null && hl.endOffset! > hl.startOffset!) {
      if (hl.startOffset! < block.endOffset && hl.endOffset! > block.startOffset) {
        segStart = math.max(0, hl.startOffset! - block.startOffset);
        segEnd = math.min(block.text.length, hl.endOffset! - block.startOffset);
      }
    }

    // 2. Substring fallback matching
    if (segStart == null || segEnd == null || segStart >= segEnd) {
      if (hl.selectedText.isNotEmpty) {
        final idx = block.text.indexOf(hl.selectedText);
        if (idx != -1) {
          segStart = idx;
          segEnd = idx + hl.selectedText.length;
        } else {
          final lines = hl.selectedText.split(RegExp(r'\n+'));
          for (final line in lines) {
            final trimmedLine = line.trim();
            if (trimmedLine.isNotEmpty && block.text.contains(trimmedLine)) {
              final lineIdx = block.text.indexOf(trimmedLine);
              segStart = lineIdx;
              segEnd = lineIdx + trimmedLine.length;
              break;
            }
          }
        }
      }
    }

    if (segStart != null && segEnd != null && segStart < segEnd && segStart < block.text.length) {
      final clampedEnd = math.min(block.text.length, segEnd);
      segments.add(_HighlightSegment(
        start: segStart,
        end: clampedEnd,
        highlight: hl,
      ));
    }
  }

  if (segments.isEmpty) {
    return parseInlineSpans(block.text, baseStyle, accentColor);
  }

  segments.sort((a, b) => a.start.compareTo(b.start));

  final spans = <InlineSpan>[];
  int currentIndex = 0;

  for (final seg in segments) {
    if (seg.start > currentIndex) {
      spans.addAll(parseInlineSpans(
        block.text.substring(currentIndex, seg.start),
        baseStyle,
        accentColor,
      ));
    }

    final segActualStart = math.max(currentIndex, seg.start);
    if (segActualStart < seg.end) {
      final chunkText = block.text.substring(segActualStart, seg.end);
      final hlColor = seg.highlight.highlightColor.resolve(settings.themeMode);
      final hlStyle = baseStyle.copyWith(backgroundColor: hlColor);
      final innerSpans = parseInlineSpans(chunkText, hlStyle, accentColor);

      for (final s in innerSpans) {
        if (s is TextSpan) {
          spans.add(TextSpan(
            text: s.text,
            children: s.children,
            style: (s.style ?? hlStyle).copyWith(backgroundColor: hlColor),
            recognizer: onHighlightTap != null
                ? (TapGestureRecognizer()..onTap = () => onHighlightTap(seg.highlight))
                : null,
          ));
        } else {
          spans.add(s);
        }
      }

      currentIndex = seg.end;
    }
  }

  if (currentIndex < block.text.length) {
    spans.addAll(parseInlineSpans(
      block.text.substring(currentIndex),
      baseStyle,
      accentColor,
    ));
  }

  return spans;
}

/// Renders a single ReaderBlock widget matching the user's typography settings and theme.
Widget buildReaderBlockWidget({
  required ReaderBlock block,
  required ReaderSettings settings,
  required ThemeData theme,
  List<Highlight> highlights = const [],
  void Function(Highlight hl)? onHighlightTap,
}) {
  final colors = theme.extension<ShelfdColors>();
  final headingColor = colors?.ink ?? theme.colorScheme.primary;
  final accentColor = colors?.accentPrimary ?? theme.colorScheme.primary;
  final onSurfaceColor = colors?.ink ?? theme.colorScheme.onSurface;
  final secondaryColor = colors?.secondaryInk ?? onSurfaceColor.withValues(alpha: 0.85);
  final dividerColor = colors?.subtleBorder ?? theme.dividerColor;

  switch (block.type) {
    case ReaderBlockType.h1:
      final style = AppTypography.titleSerif(
        fontSize: (settings.fontSize * 1.5).clamp(24.0, 34.0),
        fontWeight: FontWeight.bold,
        color: headingColor,
      );
      return Padding(
        padding: const EdgeInsets.only(top: 28, bottom: 14),
        child: Text.rich(
          TextSpan(
            children: buildBlockSpans(
              block: block,
              baseStyle: style,
              accentColor: accentColor,
              settings: settings,
              highlights: highlights,
              onHighlightTap: onHighlightTap,
            ),
          ),
        ),
      );

    case ReaderBlockType.h2:
      final style = AppTypography.titleSerif(
        fontSize: (settings.fontSize * 1.3).clamp(20.0, 28.0),
        fontWeight: FontWeight.w700,
        color: headingColor,
      );
      return Padding(
        padding: const EdgeInsets.only(top: 24, bottom: 12),
        child: Text.rich(
          TextSpan(
            children: buildBlockSpans(
              block: block,
              baseStyle: style,
              accentColor: accentColor,
              settings: settings,
              highlights: highlights,
              onHighlightTap: onHighlightTap,
            ),
          ),
        ),
      );

    case ReaderBlockType.h3:
      final style = AppTypography.titleSerif(
        fontSize: (settings.fontSize * 1.15).clamp(18.0, 24.0),
        fontWeight: FontWeight.w600,
        color: headingColor,
      );
      return Padding(
        padding: const EdgeInsets.only(top: 20, bottom: 10),
        child: Text.rich(
          TextSpan(
            children: buildBlockSpans(
              block: block,
              baseStyle: style,
              accentColor: accentColor,
              settings: settings,
              highlights: highlights,
              onHighlightTap: onHighlightTap,
            ),
          ),
        ),
      );

    case ReaderBlockType.h4:
      final style = AppTypography.titleSerif(
        fontSize: settings.fontSize,
        fontWeight: FontWeight.w600,
        color: headingColor,
      );
      return Padding(
        padding: const EdgeInsets.only(top: 16, bottom: 8),
        child: Text.rich(
          TextSpan(
            children: buildBlockSpans(
              block: block,
              baseStyle: style,
              accentColor: accentColor,
              settings: settings,
              highlights: highlights,
              onHighlightTap: onHighlightTap,
            ),
          ),
        ),
      );

    case ReaderBlockType.divider:
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: AppTokens.space24),
        child: Center(
          child: SizedBox(
            width: 80,
            child: Divider(
              color: dividerColor,
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
        color: secondaryColor,
      ).copyWith(fontStyle: FontStyle.italic);

      return Container(
        margin: const EdgeInsets.only(bottom: AppTokens.space16),
        padding: const EdgeInsets.only(left: AppTokens.space16, top: 4, bottom: 4),
        decoration: BoxDecoration(
          border: Border(
            left: BorderSide(
              color: accentColor.withValues(alpha: 0.55),
              width: 3,
            ),
          ),
        ),
        child: Text.rich(
          TextSpan(
            children: buildBlockSpans(
              block: block,
              baseStyle: quoteStyle,
              accentColor: accentColor,
              settings: settings,
              highlights: highlights,
              onHighlightTap: onHighlightTap,
            ),
          ),
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
          TextSpan(
            children: buildBlockSpans(
              block: block,
              baseStyle: bodyStyle,
              accentColor: accentColor,
              settings: settings,
              highlights: highlights,
              onHighlightTap: onHighlightTap,
            ),
          ),
        ),
      );
  }
}
