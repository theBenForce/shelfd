import 'package:flutter/material.dart';
import '../../ui/core/theme.dart';

enum KindleHighlightColor {
  yellow(
    name: 'yellow',
    label: 'Yellow',
    color: Color(0xFFF9E88B),
    cardColor: Color(0xFFFFF3CD),
    darkColor: Color(0xFF8D7300),
  ),
  blue(
    name: 'blue',
    label: 'Blue',
    color: Color(0xFFBEE3F8),
    cardColor: Color(0xFFCFE2FF),
    darkColor: Color(0xFF1565C0),
  ),
  pink(
    name: 'pink',
    label: 'Pink',
    color: Color(0xFFFBB6CE),
    cardColor: Color(0xFFF8D7DA),
    darkColor: Color(0xFFC2185B),
  ),
  orange(
    name: 'orange',
    label: 'Orange',
    color: Color(0xFFFEEBC8),
    cardColor: Color(0xFFFFE0B2),
    darkColor: Color(0xFFE65100),
  );

  final String name;
  final String label;
  final Color color;
  final Color cardColor;
  final Color darkColor;

  const KindleHighlightColor({
    required this.name,
    required this.label,
    required this.color,
    required this.cardColor,
    required this.darkColor,
  });

  static KindleHighlightColor fromName(String? name) {
    return KindleHighlightColor.values.firstWhere(
      (c) => c.name == name?.toLowerCase(),
      orElse: () => KindleHighlightColor.yellow,
    );
  }

  Color resolve(ReadingThemeMode themeMode) {
    switch (themeMode) {
      case ReadingThemeMode.dark:
        return darkColor.withValues(alpha: 0.55);
      case ReadingThemeMode.sepia:
        return color.withValues(alpha: 0.55);
      case ReadingThemeMode.bone:
        return color.withValues(alpha: 0.50);
    }
  }
}

class Highlight {
  final String id;
  final String bookId;
  final String? chapterId;
  final String selectedText;
  final String? note;
  final String color;
  final int? startOffset;
  final int? endOffset;
  final int? startParagraph;
  final int? endParagraph;
  final String? location;
  final DateTime? createdAt;

  const Highlight({
    required this.id,
    required this.bookId,
    this.chapterId,
    required this.selectedText,
    this.note,
    this.color = 'yellow',
    this.startOffset,
    this.endOffset,
    this.startParagraph,
    this.endParagraph,
    this.location,
    this.createdAt,
  });

  KindleHighlightColor get highlightColor => KindleHighlightColor.fromName(color);

  factory Highlight.fromJson(Map<String, dynamic> json) {
    return Highlight(
      id: json['id'] as String? ?? '',
      bookId: json['book_id'] as String? ?? '',
      chapterId: json['chapter_id'] as String?,
      selectedText: json['selected_text'] as String? ?? '',
      note: json['note'] as String?,
      color: json['color'] as String? ?? 'yellow',
      startOffset: json['start_offset'] as int?,
      endOffset: json['end_offset'] as int?,
      startParagraph: json['start_paragraph'] as int?,
      endParagraph: json['end_paragraph'] as int?,
      location: json['location'] as String?,
      createdAt: json['created_at'] != null ? DateTime.tryParse(json['created_at'] as String) : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'book_id': bookId,
      if (chapterId != null) 'chapter_id': chapterId,
      'selected_text': selectedText,
      if (note != null) 'note': note,
      'color': color,
      if (startOffset != null) 'start_offset': startOffset,
      if (endOffset != null) 'end_offset': endOffset,
      if (startParagraph != null) 'start_paragraph': startParagraph,
      if (endParagraph != null) 'end_paragraph': endParagraph,
      if (location != null) 'location': location,
      if (createdAt != null) 'created_at': createdAt!.toIso8601String(),
    };
  }

  Highlight copyWith({
    String? id,
    String? bookId,
    String? chapterId,
    String? selectedText,
    String? note,
    String? color,
    int? startOffset,
    int? endOffset,
    int? startParagraph,
    int? endParagraph,
    String? location,
    DateTime? createdAt,
  }) {
    return Highlight(
      id: id ?? this.id,
      bookId: bookId ?? this.bookId,
      chapterId: chapterId ?? this.chapterId,
      selectedText: selectedText ?? this.selectedText,
      note: note ?? this.note,
      color: color ?? this.color,
      startOffset: startOffset ?? this.startOffset,
      endOffset: endOffset ?? this.endOffset,
      startParagraph: startParagraph ?? this.startParagraph,
      endParagraph: endParagraph ?? this.endParagraph,
      location: location ?? this.location,
      createdAt: createdAt ?? this.createdAt,
    );
  }
}
