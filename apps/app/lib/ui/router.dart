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
        return '/library';
      }
      return null;
    },
    routes: [
      GoRoute(
        path: '/connect',
        builder: (context, state) => const ConnectView(),
      ),

      // ShellRoute: wraps primary tabs (/library, /search, /settings) in persistent AppShell
      ShellRoute(
        builder: (context, state, child) => AppShell(child: child),
        routes: [
          GoRoute(
            path: '/library',
            builder: (context, state) => const LibraryView(),
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

      // Series hierarchy with subroutes
      GoRoute(
        path: '/series',
        redirect: (context, state) =>
            state.uri.path == '/series' ? '/library?filter=series' : null,
        routes: [
          GoRoute(
            path: ':seriesId',
            builder: (context, state) {
              final seriesId = state.pathParameters['seriesId'] ?? '';
              final seriesName = state.uri.queryParameters['name'];
              return SeriesDetailView(
                seriesId: seriesId,
                seriesName: seriesName,
              );
            },
          ),
        ],
      ),

      // Author hierarchy with subroutes
      GoRoute(
        path: '/author',
        redirect: (context, state) =>
            state.uri.path == '/author' ? '/library?filter=authors' : null,
        routes: [
          GoRoute(
            path: ':authorId',
            builder: (context, state) {
              final authorId = state.pathParameters['authorId'] ?? '';
              final authorName = state.uri.queryParameters['name'];
              return AuthorDetailView(
                authorId: authorId,
                authorName: authorName,
              );
            },
          ),
        ],
      ),
      GoRoute(
        path: '/authors/:authorId',
        redirect: (context, state) => '/author/${state.pathParameters['authorId']}',
      ),

      // Book hierarchy with nested subroutes for reading and chapters
      GoRoute(
        path: '/book/:bookId',
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

      // Backwards compatibility redirects for legacy /reader routes
      GoRoute(
        path: '/reader/:bookId',
        redirect: (context, state) => '/book/${state.pathParameters['bookId']}',
      ),
      GoRoute(
        path: '/reader/:bookId/read',
        redirect: (context, state) => '/book/${state.pathParameters['bookId']}/read',
      ),
      GoRoute(
        path: '/reader/:bookId/read/:chapterIdentifier',
        redirect: (context, state) =>
            '/book/${state.pathParameters['bookId']}/read/${state.pathParameters['chapterIdentifier']}',
      ),
      GoRoute(
        path: '/reader/:bookId/:chapterIdentifier',
        redirect: (context, state) =>
            '/book/${state.pathParameters['bookId']}/${state.pathParameters['chapterIdentifier']}',
      ),
    ],
    errorBuilder: (context, state) => Scaffold(
      body: Center(
        child: Text('Page not found: ${state.uri}'),
      ),
    ),
  );
}
