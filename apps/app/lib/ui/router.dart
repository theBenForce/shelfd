import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../data/services/storage_service.dart';
import 'core/app_shell.dart';
import 'features/authors/author_detail_view.dart';
import 'features/book_detail/book_detail_view.dart';
import 'features/connect/connect_view.dart';
import 'features/library/library_view.dart';
import 'features/reader/reader_view.dart';
import 'features/search/search_view.dart';
import 'features/series/series_detail_view.dart';
import 'features/settings/settings_view.dart';

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
        return '/books';
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
        redirect: (context, state) {
          final query = state.uri.hasQuery ? '?${state.uri.query}' : '';
          return '/books$query';
        },
      ),

      // ShellRoute: wraps primary tabs (/books, /series, /authors, /search, /settings) in persistent AppShell
      ShellRoute(
        builder: (context, state, child) => AppShell(child: child),
        routes: [
          GoRoute(
            path: '/books',
            builder: (context, state) {
              final filter = state.uri.queryParameters['filter'];
              return LibraryView(
                mode: LibraryViewMode.books,
                initialFilter: filter,
              );
            },
          ),
          GoRoute(
            path: '/series',
            builder: (context, state) => const LibraryView(
              mode: LibraryViewMode.series,
            ),
          ),
          GoRoute(
            path: '/authors',
            builder: (context, state) => const LibraryView(
              mode: LibraryViewMode.authors,
            ),
          ),
          GoRoute(
            path: '/search',
            builder: (context, state) => const SearchView(),
          ),
          GoRoute(
            path: '/settings',
            builder: (context, state) => const SettingsView(),
          ),
        ],
      ),

      // Series detail route
      GoRoute(
        path: '/series/:seriesId',
        builder: (context, state) {
          final seriesId = state.pathParameters['seriesId'] ?? '';
          final seriesName = state.uri.queryParameters['name'];
          return SeriesDetailView(
            seriesId: seriesId,
            seriesName: seriesName,
          );
        },
      ),

      // Author hierarchy with subroutes and redirects
      GoRoute(
        path: '/author',
        redirect: (context, state) => '/authors',
      ),
      GoRoute(
        path: '/author/:authorId',
        redirect: (context, state) {
          final query = state.uri.hasQuery ? '?${state.uri.query}' : '';
          return '/authors/${state.pathParameters['authorId']}$query';
        },
      ),
      GoRoute(
        path: '/authors/:authorId',
        builder: (context, state) {
          final authorId = state.pathParameters['authorId'] ?? '';
          final authorName = state.uri.queryParameters['name'];
          return AuthorDetailView(
            authorId: authorId,
            authorName: authorName,
          );
        },
      ),

      // Book hierarchy with nested subroutes for reading and chapters
      GoRoute(
        path: '/books/:bookId',
        builder: (context, state) {
          final bookId = state.pathParameters['bookId'] ?? '';
          return BookDetailView(
            bookId: bookId,
          );
        },
        routes: [
          GoRoute(
            path: 'read',
            builder: (context, state) {
              final bookId = state.pathParameters['bookId'] ?? '';
              return ReaderView(
                bookId: bookId,
              );
            },
            routes: [
              GoRoute(
                path: ':chapterIdentifier',
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
          ),
          GoRoute(
            path: ':chapterIdentifier',
            builder: (context, state) {
              final bookId = state.pathParameters['bookId'] ?? '';
              final chapterIdentifier = state.pathParameters['chapterIdentifier'];
              if (chapterIdentifier == 'read') {
                return ReaderView(bookId: bookId);
              }
              return ReaderView(
                bookId: bookId,
                chapterIdentifier: chapterIdentifier,
              );
            },
          ),
        ],
      ),

      // Backwards compatibility redirects for legacy /book routes
      GoRoute(
        path: '/book/:bookId',
        redirect: (context, state) => '/books/${state.pathParameters['bookId']}',
      ),
      GoRoute(
        path: '/book/:bookId/read',
        redirect: (context, state) => '/books/${state.pathParameters['bookId']}/read',
      ),
      GoRoute(
        path: '/book/:bookId/read/:chapterIdentifier',
        redirect: (context, state) =>
            '/books/${state.pathParameters['bookId']}/read/${state.pathParameters['chapterIdentifier']}',
      ),
      GoRoute(
        path: '/book/:bookId/:chapterIdentifier',
        redirect: (context, state) =>
            '/books/${state.pathParameters['bookId']}/${state.pathParameters['chapterIdentifier']}',
      ),

      // Backwards compatibility redirects for legacy /reader routes
      GoRoute(
        path: '/reader/:bookId',
        redirect: (context, state) => '/books/${state.pathParameters['bookId']}',
      ),
      GoRoute(
        path: '/reader/:bookId/read',
        redirect: (context, state) => '/books/${state.pathParameters['bookId']}/read',
      ),
      GoRoute(
        path: '/reader/:bookId/read/:chapterIdentifier',
        redirect: (context, state) =>
            '/books/${state.pathParameters['bookId']}/read/${state.pathParameters['chapterIdentifier']}',
      ),
      GoRoute(
        path: '/reader/:bookId/:chapterIdentifier',
        redirect: (context, state) =>
            '/books/${state.pathParameters['bookId']}/${state.pathParameters['chapterIdentifier']}',
      ),
    ],
    errorBuilder: (context, state) => Scaffold(
      body: Center(
        child: Text('Page not found: ${state.uri}'),
      ),
    ),
  );
}
