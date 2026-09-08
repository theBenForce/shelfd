class BookCitation {
  final String chapterId;
  final int chapterIndex;
  final String? chapterTitle;
  final String summary;

  const BookCitation({
    required this.chapterId,
    required this.chapterIndex,
    this.chapterTitle,
    required this.summary,
  });

  factory BookCitation.fromJson(Map<String, dynamic> json) {
    return BookCitation(
      chapterId: json['chapter_id'] as String? ?? '',
      chapterIndex: (json['chapter_index'] as num?)?.toInt() ?? 0,
      chapterTitle: json['chapter_title'] as String?,
      summary: json['summary'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'chapter_id': chapterId,
      'chapter_index': chapterIndex,
      if (chapterTitle != null) 'chapter_title': chapterTitle,
      'summary': summary,
    };
  }
}

class BookChatMessage {
  final String role; // 'user', 'assistant', 'system'
  final String content;
  final List<BookCitation> citations;
  final DateTime timestamp;

  BookChatMessage({
    required this.role,
    required this.content,
    this.citations = const [],
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  factory BookChatMessage.fromJson(Map<String, dynamic> json) {
    return BookChatMessage(
      role: json['role'] as String? ?? 'user',
      content: json['content'] as String? ?? '',
      citations: (json['citations'] as List<dynamic>? ?? [])
          .whereType<Map<String, dynamic>>()
          .map((c) => BookCitation.fromJson(c))
          .toList(),
      timestamp: json['timestamp'] != null
          ? DateTime.tryParse(json['timestamp'] as String) ?? DateTime.now()
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'role': role,
      'content': content,
      if (citations.isNotEmpty) 'citations': citations.map((c) => c.toJson()).toList(),
      'timestamp': timestamp.toIso8601String(),
    };
  }
}

class BookChatResponse {
  final String reply;
  final List<BookCitation> citations;

  const BookChatResponse({
    required this.reply,
    this.citations = const [],
  });

  factory BookChatResponse.fromJson(Map<String, dynamic> json) {
    return BookChatResponse(
      reply: json['reply'] as String? ?? json['response'] as String? ?? '',
      citations: (json['citations'] as List<dynamic>? ?? [])
          .whereType<Map<String, dynamic>>()
          .map((c) => BookCitation.fromJson(c))
          .toList(),
    );
  }
}
