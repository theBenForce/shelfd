import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../data/services/storage_service.dart';
import 'features/connect/connect_view.dart';
import 'features/library/library_view.dart';
import 'features/reader/reader_view.dart';
import 'features/search/search_view.dart';

GoRouter createRouter({required String initialLocation, StorageService? storageService}) {
  return GoRouter(
    initialLocation: initialLocation,
    redirect: (context, state) {
      if (storageService == null) return null;
      final token = storageService.getAuthToken();
      final hasToken = token != null && token.isNotEmpty;
      final isConnect = state.matchedLocation == '/connect';

      if (!hasToken && !isConnect) {
        return '/connect';
      }
      if (hasToken && isConnect) {
        return '/library';
      }
      return null;
    },
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
        path: '/reader/:bookId',
        builder: (context, state) {
          final bookId = state.pathParameters['bookId'] ?? '';
          return ReaderView(
            bookId: bookId,
          );
        },
      ),
      GoRoute(
        path: '/reader/:bookId/:chapterIdentifier',
        builder: (context, state) {
          final bookId = state.pathParameters['bookId'] ?? '';
          final chapterIdentifier = state.pathParameters['chapterIdentifier'];
          return ReaderView(
            bookId: bookId,
            chapterIdentifier: chapterIdentifier,
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
