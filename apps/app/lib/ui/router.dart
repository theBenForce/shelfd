import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'features/connect/connect_view.dart';
import 'features/library/library_view.dart';
import 'features/reader/reader_view.dart';
import 'features/search/search_view.dart';

GoRouter createRouter({required String initialLocation}) {
  return GoRouter(
    initialLocation: initialLocation,
    routes: [
      GoRoute(
        path: '/connect',
        builder: (context, state) => const ConnectView(),
      ),
      GoRoute(
        path: '/library',
        builder: (context, state) => const LibraryView(),
      ),
      GoRoute(
        path: '/search',
        builder: (context, state) => const SearchView(),
      ),
      GoRoute(
        path: '/reader/:bookId/:chapterIndex',
        builder: (context, state) {
          final bookId = state.pathParameters['bookId'] ?? '';
          final chapterIndex = int.tryParse(state.pathParameters['chapterIndex'] ?? '0') ?? 0;
          return ReaderView(
            bookId: bookId,
            chapterIndex: chapterIndex,
          );
        },
      ),
    ],
    errorBuilder: (context, state) => Scaffold(
      body: Center(
        child: Text('Page not found: ${state.uri}'),
      ),
    ),
  );
}
