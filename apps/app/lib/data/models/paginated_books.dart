import 'book.dart';

class PaginatedBooks {
  final List<Book> books;
  final int total;
  final int page;
  final int perPage;
  final int limit;
  final int offset;

  const PaginatedBooks({
    required this.books,
    required this.total,
    this.page = 1,
    this.perPage = 24,
    this.limit = 24,
    this.offset = 0,
  });

  bool get hasMore => offset + books.length < total;
}
