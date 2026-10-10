import 'dart:async';
import 'dart:js_interop';
import 'package:web/web.dart' as web;
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

bool isDirectoryPath(String path) => false;

Future<List<PickedEpubFile>> scanPathForEpubs(String path) async => [];

Future<List<PickedEpubFile>> pickFolderForEpubs() {
  final completer = Completer<List<PickedEpubFile>>();
  final input = web.document.createElement('input') as web.HTMLInputElement;
  input.type = 'file';
  input.setAttribute('webkitdirectory', '');
  input.setAttribute('directory', '');
  input.multiple = true;
  input.style.display = 'none';

  web.document.body?.appendChild(input);

  void cleanup() {
    input.remove();
  }

  input.addEventListener(
    'change',
    (web.Event e) {
      final fileList = input.files;
      if (fileList == null || fileList.length == 0) {
        cleanup();
        if (!completer.isCompleted) completer.complete([]);
        return;
      }

      final filesByFolder = <String, List<web.File>>{};
      for (var i = 0; i < fileList.length; i++) {
        final file = fileList.item(i);
        if (file == null) continue;
        final relPath = file.webkitRelativePath;
        final folder = relPath.contains('/')
            ? relPath.substring(0, relPath.lastIndexOf('/'))
            : '';
        filesByFolder.putIfAbsent(folder, () => []).add(file);
      }

      final results = <PickedEpubFile>[];
      for (final entry in filesByFolder.entries) {
        final folderFiles = entry.value;
        final bookFiles = folderFiles.where((f) => isSupportedBook(f.name)).toList();
        final coverFiles = folderFiles.where((f) => isSupportedCover(f.name)).toList();

        for (final bookFile in bookFiles) {
          final bookName = bookFile.name;
          final bookBase = bookName.contains('.')
              ? bookName.substring(0, bookName.lastIndexOf('.')).toLowerCase()
              : bookName.toLowerCase();

          web.File? matchedCover;
          for (final c in coverFiles) {
            final cBase = c.name.contains('.')
                ? c.name.substring(0, c.name.lastIndexOf('.')).toLowerCase()
                : c.name.toLowerCase();
            if (cBase == bookBase) {
              matchedCover = c;
              break;
            }
          }

          if (matchedCover == null) {
            for (final c in coverFiles) {
              final cBase = c.name.contains('.')
                  ? c.name.substring(0, c.name.lastIndexOf('.')).toLowerCase()
                  : c.name.toLowerCase();
              if (cBase == 'cover' || cBase == 'folder') {
                matchedCover = c;
                break;
              }
            }
          }

          if (matchedCover == null && coverFiles.length == 1) {
            matchedCover = coverFiles.first;
          }

          String? coverName;
          Future<List<int>> Function()? readCover;
          if (matchedCover != null) {
            coverName = matchedCover.name;
            final finalCover = matchedCover;
            readCover = () async {
              final arrayBuffer = await finalCover.arrayBuffer().toDart;
              return arrayBuffer.toDart.asUint8List();
            };
          }

          results.add(PickedEpubFile(
            name: bookName,
            readBytes: () async {
              final arrayBuffer = await bookFile.arrayBuffer().toDart;
              return arrayBuffer.toDart.asUint8List();
            },
            coverName: coverName,
            readCoverBytes: readCover,
          ));
        }
      }

      cleanup();
      if (!completer.isCompleted) completer.complete(results);
    }.toJS,
  );

  input.addEventListener(
    'cancel',
    (web.Event e) {
      cleanup();
      if (!completer.isCompleted) completer.complete([]);
    }.toJS,
  );

  input.click();

  return completer.future;
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
