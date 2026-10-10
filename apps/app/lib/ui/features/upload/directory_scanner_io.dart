import 'dart:io';
import 'package:file_picker/file_picker.dart';
import '../../state/upload_provider.dart';

const supportedBookExtensions = {'.epub', '.m4b', '.mp3', '.m4a', '.flac'};
const supportedCoverExtensions = {'.jpg', '.jpeg', '.png', '.webp'};

bool isSupportedBook(String path) {
  final lower = path.toLowerCase();
  return supportedBookExtensions.any((ext) => lower.endsWith(ext));
}

bool isSupportedCover(String path) {
  final lower = path.toLowerCase();
  return supportedCoverExtensions.any((ext) => lower.endsWith(ext));
}

bool isDirectoryPath(String path) {
  try {
    return FileSystemEntity.typeSync(path) == FileSystemEntityType.directory;
  } catch (_) {
    return false;
  }
}

File? _findMatchingCover(String bookFilename, List<File> coverFiles) {
  if (coverFiles.isEmpty) return null;

  final bookBase = bookFilename.contains('.')
      ? bookFilename.substring(0, bookFilename.lastIndexOf('.')).toLowerCase()
      : bookFilename.toLowerCase();

  // 1. Exact base name match (e.g. MyBook.jpg next to MyBook.m4b)
  for (final c in coverFiles) {
    final cName = c.uri.pathSegments.isNotEmpty
        ? c.uri.pathSegments.last
        : c.path.split(Platform.pathSeparator).last;
    final cBase = cName.contains('.')
        ? cName.substring(0, cName.lastIndexOf('.')).toLowerCase()
        : cName.toLowerCase();
    if (cBase == bookBase) {
      return c;
    }
  }

  // 2. Standard cover / folder filenames
  for (final c in coverFiles) {
    final cName = (c.uri.pathSegments.isNotEmpty
            ? c.uri.pathSegments.last
            : c.path.split(Platform.pathSeparator).last)
        .toLowerCase();
    final cBase = cName.contains('.') ? cName.substring(0, cName.lastIndexOf('.')) : cName;
    if (cBase == 'cover' || cBase == 'folder') {
      return c;
    }
  }

  // 3. Lone image in folder
  if (coverFiles.length == 1) {
    return coverFiles.first;
  }

  return null;
}

Future<List<PickedEpubFile>> scanPathForEpubs(String path) async {
  final results = <PickedEpubFile>[];
  try {
    final type = FileSystemEntity.typeSync(path);
    if (type == FileSystemEntityType.directory) {
      final dir = Directory(path);
      final entities = dir.listSync(recursive: true, followLinks: false);

      final filesByDir = <String, List<File>>{};
      for (final entity in entities) {
        if (entity is File) {
          final parentPath = entity.parent.path;
          filesByDir.putIfAbsent(parentPath, () => []).add(entity);
        }
      }

      for (final dirFiles in filesByDir.values) {
        final bookFiles = dirFiles.where((f) => isSupportedBook(f.path)).toList();
        final coverFiles = dirFiles.where((f) => isSupportedCover(f.path)).toList();

        for (final bookFile in bookFiles) {
          final bookFilename = bookFile.uri.pathSegments.isNotEmpty
              ? bookFile.uri.pathSegments.last
              : bookFile.path.split(Platform.pathSeparator).last;

          final matchedCover = _findMatchingCover(bookFilename, coverFiles);
          String? coverFilename;
          Future<List<int>> Function()? readCover;

          if (matchedCover != null) {
            coverFilename = matchedCover.uri.pathSegments.isNotEmpty
                ? matchedCover.uri.pathSegments.last
                : matchedCover.path.split(Platform.pathSeparator).last;
            final finalCover = matchedCover;
            readCover = () => finalCover.readAsBytes();
          }

          results.add(PickedEpubFile(
            name: bookFilename,
            path: bookFile.path,
            readBytes: () => bookFile.readAsBytes(),
            coverName: coverFilename,
            readCoverBytes: readCover,
          ));
        }
      }
    } else if (type == FileSystemEntityType.file && isSupportedBook(path)) {
      final file = File(path);
      final filename = file.uri.pathSegments.isNotEmpty
          ? file.uri.pathSegments.last
          : path.split(Platform.pathSeparator).last;

      File? matchedCover;
      try {
        final parentDir = file.parent;
        if (parentDir.existsSync()) {
          final siblings = parentDir.listSync(followLinks: false);
          final coverSiblings = siblings.whereType<File>().where((f) => isSupportedCover(f.path)).toList();
          matchedCover = _findMatchingCover(filename, coverSiblings);
        }
      } catch (_) {}

      String? coverFilename;
      Future<List<int>> Function()? readCover;
      if (matchedCover != null) {
        coverFilename = matchedCover.uri.pathSegments.isNotEmpty
            ? matchedCover.uri.pathSegments.last
            : matchedCover.path.split(Platform.pathSeparator).last;
        final finalCover = matchedCover;
        readCover = () => finalCover.readAsBytes();
      }

      results.add(PickedEpubFile(
        name: filename,
        path: path,
        readBytes: () => file.readAsBytes(),
        coverName: coverFilename,
        readCoverBytes: readCover,
      ));
    }
  } catch (_) {}
  return results;
}

Future<List<PickedEpubFile>> pickFolderForEpubs() async {
  final dirPath = await FilePicker.getDirectoryPath();
  if (dirPath == null || dirPath.trim().isEmpty) {
    return [];
  }
  return scanPathForEpubs(dirPath);
}

List<PickedEpubFile> matchLooseFilesWithCovers(
  List<PickedEpubFile> books,
  List<PickedEpubFile> covers,
) {
  if (covers.isEmpty) return books;

  final matched = <PickedEpubFile>[];
  for (final book in books) {
    if (book.coverName != null || book.coverBytes != null || book.readCoverBytes != null) {
      matched.add(book);
      continue;
    }

    final bookBase = book.name.contains('.')
        ? book.name.substring(0, book.name.lastIndexOf('.')).toLowerCase()
        : book.name.toLowerCase();

    PickedEpubFile? matchedCover;
    for (final c in covers) {
      final cBase = c.name.contains('.')
          ? c.name.substring(0, c.name.lastIndexOf('.')).toLowerCase()
          : c.name.toLowerCase();
      if (cBase == bookBase) {
        matchedCover = c;
        break;
      }
    }

    if (matchedCover == null) {
      for (final c in covers) {
        final cBase = c.name.contains('.')
            ? c.name.substring(0, c.name.lastIndexOf('.')).toLowerCase()
            : c.name.toLowerCase();
        if (cBase == 'cover' || cBase == 'folder') {
          matchedCover = c;
          break;
        }
      }
    }

    if (matchedCover == null && covers.length == 1) {
      matchedCover = covers.first;
    }

    if (matchedCover != null) {
      matched.add(PickedEpubFile(
        name: book.name,
        path: book.path,
        bytes: book.bytes,
        readBytes: book.readBytes,
        coverName: matchedCover.name,
        coverBytes: matchedCover.bytes,
        readCoverBytes: matchedCover.readBytes,
      ));
    } else {
      matched.add(book);
    }
  }
  return matched;
}
