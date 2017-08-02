package service

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/goushuyun/taobao-erp/book/db"
	"github.com/goushuyun/taobao-erp/misc/bookspider"
	"github.com/pborman/uuid"

	"golang.org/x/net/context"

	"github.com/goushuyun/taobao-erp/errs"
	"github.com/goushuyun/taobao-erp/misc"
	"github.com/goushuyun/taobao-erp/pb"

	"github.com/wothing/log"
)

type BookServer struct {
}

//获取图书信息
func (s *BookServer) GetBookInfo(ctx context.Context, in *pb.Book) (*pb.BookListResp, error) {
	tid := misc.GetTidFromContext(ctx)
	defer log.TraceOut(log.TraceIn(tid, "GetBookInfo", "%#v", in))
	/*
	   check if need precision search by book'id if id not null ,just search book info from local db
	*/
	if in.Id != "" {
		// get book info from local db
		books, err := db.GetBookInfo(in)
		if err != nil {
			log.Error(err)
			return nil, errs.Wrap(errors.New(err.Error()))
		}
		if len(books) <= 0 {
			return &pb.BookListResp{Code: errs.Ok, Message: "errParam"}, nil
		} else {
			return &pb.BookListResp{Code: errs.Ok, Message: "ok", Data: books}, nil
		}
	} else {
		if in.Isbn != "" {
			// first : get get book info from local db
			books, err := db.GetBookInfo(in)
			if err != nil {
				log.Error(err)
				return nil, errs.Wrap(errors.New(err.Error()))
			}
			if len(books) > 0 {
				return &pb.BookListResp{Code: errs.Ok, Message: "ok", Data: books}, nil
			}
			// second :if local db don't has this book info ,just get it from internet (dangdang ,jd ,book uu)
			// this function distinguish the upload mode: default or speed
			book, err := insertByUploadMode(in.Isbn, in.UploadMode)
			if err != nil {
				log.Error(err)
				return nil, errs.Wrap(errors.New(err.Error()))
			}
			bookresp := &pb.BookListResp{Code: errs.Ok, Message: "ok"}
			bookresp.Data = append(bookresp.Data, book)
			return bookresp, nil
		}
	}
	return &pb.BookListResp{Code: errs.Ok, Message: "errParam"}, nil
}

//获取图书信息
func (s *BookServer) GetLocalBookInfo(ctx context.Context, in *pb.Book) (*pb.BookListResp, error) {
	tid := misc.GetTidFromContext(ctx)
	defer log.TraceOut(log.TraceIn(tid, "GetLocalBookInfo", "%#v", in))

	// get book info from local db
	books, err := db.GetBookInfo(in)
	if err != nil {
		log.Error(err)
		return nil, errs.Wrap(errors.New(err.Error()))
	}
	if len(books) <= 0 {
		return &pb.BookListResp{Code: errs.Ok, Message: "errParam"}, nil
	}
	return &pb.BookListResp{Code: errs.Ok, Message: "ok", Data: books}, nil

}

//change the book info
func (s *BookServer) UpdateBookInfo(ctx context.Context, in *pb.Book) (*pb.BookResp, error) {
	tid := misc.GetTidFromContext(ctx)
	defer log.TraceOut(log.TraceIn(tid, "UpdateBookInfo", "%#v", in))
	updateContent, err := db.UpdateBookInfo(in)
	if err != nil {
		log.Error(err)
		return nil, errs.Wrap(errors.New(err.Error()))
	}
	log.Debug(updateContent)
	return &pb.BookResp{Code: errs.Ok, Message: "ok"}, nil
}

//insert new data to book
func (s *BookServer) SaveBook(ctx context.Context, in *pb.Book) (*pb.BookResp, error) {
	tid := misc.GetTidFromContext(ctx)
	defer log.TraceOut(log.TraceIn(tid, "SaveBook", "%#v", in))
	err := db.InsertBookInfo(in)
	if err != nil {
		//check the err reason if equal 'exists' in particular.if yes ,return specially identification str
		if err.Error() == "exists" {
			return &pb.BookResp{Code: errs.Ok, Message: "exists", Data: in}, nil
		}
		log.Error(err)
		return nil, errs.Wrap(errors.New(err.Error()))
	}
	return &pb.BookResp{Code: errs.Ok, Message: "ok", Data: in}, nil
}

/*
	private function: handle the book info
	 1: download and upload the book image to qiniu
	 2: handle the book title
*/
func handleBookInfos(book *pb.Book, ctx context.Context) error {
	t := time.Now()
	timestamp := t.Format("20060102030405")
	if strings.HasPrefix(book.Image, "http") {
		fetchImageReq := &pb.FetchImageReq{
			Zone: pb.MediaZone_Test,
			Url:  book.Image,
			Key:  timestamp + book.Isbn + filepath.Ext(book.Image),
		}
		mediaResp := &pb.FetchImageResp{}
		err := misc.CallSVC(ctx, "mediastore", "FetchImage", fetchImageReq, mediaResp)
		if err != nil {
			log.Error(err)
			return err
		}
		book.Image = fetchImageReq.Key
	}
	return nil
}

/*
	private function: in order to resolve the condition that user wantna upload book quick
	 if upload mode is 1 , so speed upload ,just omit the wait time about the search book info on internet
	 if upload mode is 0, wait until search over
*/

func insertByUploadMode(isbn string, uploadMode int64) (book *pb.Book, err error) {
	ctx := metadata.NewContext(context.Background(), metadata.Pairs("tid", uuid.New()))
	if uploadMode == 0 {
		book, err = bookspider.GetBookInfoBySpider(isbn, "")
		if err != nil {
			log.Error(err)
			return
		}
		if book != nil {
			err = handleBookInfos(book, ctx) //handle the book info
			if err != nil {
				log.Error(err)
				return
			}
		} else {
			//if book is not found from internet just init a book struct with one field 'isbn'
			book = &pb.Book{Isbn: isbn}
		}
		//finally : insert a new data and return
		err = db.InsertBookInfo(book)
		if err != nil {
			log.Error(err)
			return
		}
		return
	} else {
		book = &pb.Book{Isbn: isbn}
		err = db.InsertBookInfo(book)
		if err != nil {
			log.Error(err)
			return
		}
		go func() {
			id := book.Id
			book, err = bookspider.GetBookInfoBySpider(isbn, "")
			if err != nil {
				log.Error(err)
				return
			}
			if book != nil {
				err = handleBookInfos(book, ctx) //handle the book info
				if err != nil {
					log.Error(err)
					return
				}
				book.Id = id
				db.UpdateBookInfo(book)
				return
			}
		}()

	}

	return
}